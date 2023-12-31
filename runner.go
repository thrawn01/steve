package steve

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/mailgun/holster/v4/collections"
	"github.com/mailgun/holster/v4/syncutil"
)

var (
	ErrJobNotFound   = errors.New("no such job found")
	ErrJobNotRunning = errors.New("job not running")
)

type jobIO struct {
	br      syncutil.Broadcaster
	writer  io.WriteCloser
	buffer  bytes.Buffer
	mutex   sync.Mutex
	started time.Time
	stopped time.Time
	id      ID
	running int64
	job     Job
}

type runner struct {
	jobs  *collections.LRUCache
	wg    syncutil.WaitGroup
	mutex sync.Mutex
}

func NewJobRunner(capacity int) Runner {
	return &runner{
		jobs: collections.NewLRUCache(capacity),
	}
}

func (r *runner) Run(ctx context.Context, job Job) (ID, error) {
	reader, writer := io.Pipe()

	j := jobIO{
		id:      ID(uuid.New().String()),
		br:      syncutil.NewBroadcaster(),
		started: time.Now(),
		writer:  writer,
		job:     job,
	}

	// Spawn a go routine to monitor job output, storing the output into the j.buffer
	r.wg.Go(func() {
		ch := make(chan []byte)
		atomic.StoreInt64(&j.running, 1)

		// Spawn a separate go routine as the read could block forever
		go func() {
			buf := make([]byte, 2024)
			for {
				n, err := reader.Read(buf)
				if err != nil {
					close(ch)
					return
				}
				out := make([]byte, n)
				copy(out, buf[:n])
				ch <- out
			}
		}()

		for {
			select {
			case line, ok := <-ch:
				if !ok {
					atomic.StoreInt64(&j.running, 0)
					j.mutex.Lock()
					j.stopped = time.Now()
					j.br.Broadcast()
					j.mutex.Unlock()
					return
				}
				j.mutex.Lock()
				j.buffer.Write(line)
				j.br.Broadcast()
				j.mutex.Unlock()
			}
		}
	})
	r.jobs.Add(j.id, &j)

	if err := job.Start(ctx, writer); err != nil {
		return "", err
	}

	for {
		if atomic.LoadInt64(&j.running) == 1 {
			break
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	return j.id, nil
}

func (r *runner) NewReader(id ID) (io.ReadCloser, error) {
	defer r.mutex.Unlock()
	r.mutex.Lock()

	obj, ok := r.jobs.Get(id)
	if !ok {
		return nil, ErrJobNotFound
	}
	j := obj.(*jobIO)

	// If the job isn't running, then copy the current buffer
	// into a read closer and return that to the caller.
	if atomic.LoadInt64(&j.running) == 0 {
		j.mutex.Lock()
		defer j.mutex.Unlock()
		buf := bytes.Buffer{}
		buf.Write(j.buffer.Bytes())
		return io.NopCloser(&buf), nil
	}

	// Create a go routine that sends all unread bytes to the reader then
	// waits for new bytes to be written to the j.buffer via the broadcaster.
	reader, writer := io.Pipe()
	r.wg.Go(func() {
		var idx = 0
		for {
			// Grab any bytes from the buffer we haven't sent to our reader
			j.mutex.Lock()
			src := j.buffer.Bytes()
			dst := make([]byte, j.buffer.Len()-idx)
			copy(dst, src[idx:j.buffer.Len()])
			j.mutex.Unlock()

			// Preform the Write() outside the mutex as it could block, and we don't
			// want to hold on to the mutex lock for long
			n, err := writer.Write(dst)
			if err != nil {
				// If the reader called Close() on the pipe
				return
			}
			idx += n

			// The job routine will broadcast when it stops the job and no
			// more bytes are available to read.
			if atomic.LoadInt64(&j.running) == 0 {
				writer.Close()
				return
			}

			// Wait for the broadcaster to tell us there are new bytes to read.
			j.br.Wait(string(j.id))

		}
	})

	return reader, nil
}

func (r *runner) Stop(ctx context.Context, id ID) error {
	defer r.mutex.Unlock()
	r.mutex.Lock()

	obj, ok := r.jobs.Get(id)
	if !ok {
		return ErrJobNotFound
	}
	j := obj.(*jobIO)

	// Ignore if already stopped
	if atomic.LoadInt64(&j.running) == 0 {
		return ErrJobNotRunning
	}

	return r.stop(ctx, j)
}

func (r *runner) stop(ctx context.Context, j *jobIO) error {
	// Stop the job
	if err := j.job.Stop(ctx); err != nil {
		return err
	}

	// Close the writer, this should tell the reading go routine to shutdown
	j.writer.Close()
	return nil
}

func (r *runner) Status(id ID) (Status, bool) {
	value, ok := r.jobs.Get(id)
	if !ok {
		return Status{}, false
	}
	return toStatus(value.(*jobIO)), true
}

func (r *runner) List() []Status {
	defer r.mutex.Unlock()
	r.mutex.Lock()

	var result []Status
	r.jobs.Each(1, func(key interface{}, value interface{}) error {
		result = append(result, toStatus(value.(*jobIO)))
		return nil
	})
	return result
}

func (r *runner) Close(ctx context.Context) error {
	defer r.mutex.Unlock()
	r.mutex.Lock()

	for _, s := range r.List() {
		obj, ok := r.jobs.Get(s.ID)
		if !ok {
			continue
		}
		j := obj.(*jobIO)
		// Skip if not running
		if atomic.LoadInt64(&j.running) == 0 {
			continue
		}
		if err := r.stop(ctx, j); err != nil {
			return fmt.Errorf("while stopping '%s': %w", j.id, err)
		}
	}
	return nil
}

func toStatus(j *jobIO) Status {
	j.mutex.Lock()
	defer j.mutex.Unlock()
	return Status{
		ID:      j.id,
		Running: atomic.LoadInt64(&j.running) == 1,
		Started: j.started,
		Stopped: j.stopped,
	}
}
