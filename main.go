package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs/v2"
	"github.com/zeebo/mwc"
)

func main() {
	ok, err := clingy.Environment{
		Root: new(root),
		Name: "shush",
		Args: os.Args[1:],
	}.Run(context.Background(), nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
	}
	if !ok || err != nil {
		os.Exit(1)
	}
}

type root struct {
	pkg string

	randoms   int
	nice      int
	taskset   int
	bench     string
	count     int
	benchtime time.Duration
}

func (r *root) Setup(params clingy.Parameters) {
	r.randoms = params.Flag("randoms", "Number of random layouts to run", 10,
		clingy.Transform(strconv.Atoi),
	).(int)
	r.nice = params.Flag("nice", "Set the process niceness", 5,
		clingy.Transform(strconv.Atoi),
	).(int)
	r.taskset = params.Flag("taskset", "Set the CPU affinity (taskset)", 1,
		clingy.Transform(strconv.Atoi),
	).(int)
	r.bench = params.Flag("bench", "Regular expression to select benchmarks to run", ".").(string)
	r.count = params.Flag("count", "Number of times to run each benchmark", 3,
		clingy.Transform(strconv.Atoi),
	).(int)
	r.benchtime = params.Flag("benchtime", "Minimum time to run each benchmark", 100*time.Millisecond,
		clingy.Transform(time.ParseDuration),
	).(time.Duration)

	r.pkg = params.Arg("package", "Package to benchmark").(string)
}

func (r *root) Execute(ctx context.Context) (err error) {
	defer func() { _ = os.Remove("shush.test") }()

	run := func(name string, args ...string) error {
		cmd := exec.Command(name, args...)
		cmd.Stdout = clingy.Stdout(ctx)
		cmd.Stderr = clingy.Stderr(ctx)
		return errs.Wrap(cmd.Run())
	}

	for range r.randoms {
		if err := run(
			"go", "test", "-c",
			fmt.Sprintf("-ldflags=-randlayout=%d", mwc.Uint32()),
			"-o", "shush.test",
			r.pkg,
		); err != nil {
			return err
		}

		if err := run(
			"sudo", "-E",
			"nice", "-n", strconv.Itoa(r.nice),
			"taskset", "-c", strconv.Itoa(r.taskset),
			"./shush.test",
			"-test.bench", r.bench,
			"-test.run", "^$",
			"-test.count", strconv.Itoa(r.count),
			"-test.benchtime", r.benchtime.String(),
		); err != nil {
			return err
		}
	}

	return nil
}
