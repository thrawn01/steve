package steve_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/mailgun/holster/v4/syncutil"
	"github.com/mailgun/holster/v4/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thrawn01/steve"
)

type testJob struct {
	wg syncutil.WaitGroup
}

func (t *testJob) Start(ctx context.Context, writer io.Writer) error {
	_, _ = fmt.Fprintf(writer, "Job Start\n")
	var count int

	t.wg.Until(func(done chan struct{}) bool {
		_, _ = fmt.Fprintf(writer, "line: %d\n", count)
		count++
		select {
		case <-done:
			_, _ = fmt.Fprintf(writer, "Job Stop\n")
			return false
		case <-time.After(time.Millisecond * 300):
			return true
		}
	})
	return nil
}

func (t *testJob) Stop(ctx context.Context) error {
	t.wg.Stop()
	return nil
}

func TestRunner(t *testing.T) {
	runner := steve.NewJobRunner(20)
	require.NotNil(t, runner)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	id, err := runner.Run(ctx, &testJob{})
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	// Supports Multiple Readers for the same job
	go func() {
		r, err := runner.NewReader(id)
		require.NoError(t, err)

		buf := bufio.NewReader(r)
		for {
			line, err := buf.ReadBytes('\n')
			if err != nil {
				return
			}
			fmt.Printf("+ GOT: %s", string(line))
		}
	}()

	go func() {
		r, err := runner.NewReader(id)
		require.NoError(t, err)

		buf := bufio.NewReader(r)
		for {
			line, err := buf.ReadBytes('\n')
			if err != nil {
				return
			}
			fmt.Printf("- GOT: %s", string(line))
		}
	}()

	// runner.Status() should eventually show the job is running
	testutil.UntilPass(t, 10, time.Millisecond*100, func(t testutil.TestingT) {
		s, ok := runner.Status(id)
		require.True(t, ok)
		assert.Equal(t, id, s.ID)
		assert.Equal(t, true, s.Running)
		assert.False(t, s.Started.IsZero())
		assert.True(t, s.Stopped.IsZero())
	})

	// Wait some time for the test to generate some output
	time.Sleep(time.Second * 2)

	// Stop the job
	err = runner.Stop(ctx, id)
	require.NoError(t, err)

	// runner.List() should eventually show the job is not running
	testutil.UntilPass(t, 10, time.Millisecond*100, func(t testutil.TestingT) {
		l := runner.List()
		assert.Equal(t, id, l[0].ID)
		assert.Equal(t, false, l[0].Running)
	})

}
