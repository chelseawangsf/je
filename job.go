package je

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
)

// Job ...
type Job struct {
	sync.RWMutex

	ID          ID        `json:"id"`
	Name        string    `json:"name"`
	Args        []string  `json:"args"`
	Interactive bool      `json:"interactive"`
	Worker      string    `json:"worker"`
	State       State     `json:"state"`
	Status      int       `json:"status"`
	CreatedAt   time.Time `json:"created"`
	StartedAt   time.Time `json:"started"`
	StoppedAt   time.Time `json:"stopped"`
	KilledAt    time.Time `json:"killed"`
	ErroredAt   time.Time `json:"errored"`

	input io.WriteCloser
	cmd   *exec.Cmd
	done  chan bool
}

func NewJob(name string, args []string, interactive bool) (job *Job, err error) {
	job = &Job{
		ID:          db.NextId(),
		Name:        name,
		Args:        args,
		Interactive: interactive,
		CreatedAt:   time.Now(),

		done: make(chan bool, 1),
	}
	err = db.Save(job)
	return
}

func (j *Job) Id() ID {
	return j.ID
}

func (j *Job) Enqueue() error {
	j.State = STATE_WAITING
	return db.Save(j)
}

func (j *Job) Start(worker string) error {
	j.Worker = worker
	j.State = STATE_RUNNING
	j.StartedAt = time.Now()
	return db.Save(j)
}

func (j *Job) Kill(force bool) (err error) {
	if force {
		err = j.cmd.Process.Kill()
		if err != nil {
			log.Errorf("error killing job #%d: %s", j.ID, err)
			return
		}

		j.done <- true
		j.State = STATE_KILLED
		j.KilledAt = time.Now()
		return db.Save(j)
	}
	return j.cmd.Process.Signal(os.Interrupt)
}

func (j *Job) Stop() error {
	j.done <- true
	j.State = STATE_STOPPED
	j.StoppedAt = time.Now()
	return db.Save(j)
}

func (j *Job) Error(err error) error {
	j.State = STATE_ERRORED
	j.ErroredAt = time.Now()
	return db.Save(j)
}

func (j *Job) Wait() {
	<-j.done
}

func (j *Job) Close() error {
	if !j.Interactive {
		return fmt.Errorf("cannot write to a non-interactive job")
	}

	return j.input.Close()
}

func (j *Job) Write(input io.Reader) (int64, error) {
	if !j.Interactive {
		return 0, fmt.Errorf("cannot write to a non-interactive job")
	}

	return io.Copy(j.input, input)
}

func (j *Job) Execute() (err error) {
	cmd := exec.Command(j.Name, j.Args...)

	if j.Interactive {
		stdin, err := cmd.StdinPipe()
		if err != nil {
			log.Errorf("error creating input for job #%d: %s", j.ID, err)
			return err
		}
		defer stdin.Close()
		j.Lock()
		j.input = stdin
		j.Unlock()
	} else {
		stdin, err := data.Read(j.ID, DATA_INPUT)
		if err != nil {
			log.Errorf("error reading input for job #%d: %s", j.ID, err)
			return err
		}
		defer stdin.Close()
		cmd.Stdin = stdin
	}

	j.Lock()
	j.cmd = cmd
	j.Unlock()

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Errorf("error reading logs from job #%d: %s", j.ID, err)
		return err
	}
	defer stderr.Close()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Errorf("error reading output from job #%d: %s", j.ID, err)
		return err
	}
	defer stdout.Close()

	logs, err := data.Write(j.ID, DATA_LOGS)
	if err != nil {
		log.Errorf("error creating logs for job #%s: %s", j.ID, err)
		return err
	}
	defer logs.Close()

	output, err := data.Write(j.ID, DATA_OUTPUT)
	if err != nil {
		log.Errorf("error creating output for job #%s: %s", j.ID, err)
		return err
	}
	defer output.Close()

	// TODO: Check if written < len(res.Log)?
	go func() {
		_, err = io.Copy(logs, stderr)
	}()

	// TODO: Check if written < len(res.Log)?
	go func() {
		_, err = io.Copy(output, stdout)
	}()

	if err = cmd.Start(); err != nil {
		log.Errorf("error starting job #%d: %s", j.ID, err)
		return err
	}

	if err := cmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			// The program has exited with an exit code != 0

			// This works on both Unix and Windows. Although package
			// syscall is generally platform dependent, WaitStatus is
			// defined for both Unix and Windows and in both cases has
			// an ExitStatus() method with the same signature.
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				j.Status = status.ExitStatus()
			}
		}
	}

	return nil
}
