package main

import (
	"bufio"
	flag "github.com/spf13/pflag"
	"fmt"
	"io"
	"os"
	"time"
)

var version = "dev"

func main() {
	startDelay := flag.Duration("start-delay", 0, "initial warm-up period before gap detection begins")
	flag.DurationVar(startDelay, "delay-start", 0, "synonym for --start-delay")
	marker := flag.String("marker", ".", "string to print during gaps")
	interval := flag.Duration("interval", 1*time.Second, "silence duration before printing a marker, and repeat rate during a gap")
	flag.DurationVar(interval, "gap", 1*time.Second, "synonym for --interval")
	fold := flag.Int("fold", 0, "print a marker every N intervals, suppressing the rest (0 = every interval)")
	max := flag.Int("max", 0, "stop printing markers after N total (default 0 = unlimited)")
	timestamp := flag.Bool("timestamp", false, "prefix each marker with the current time (15:04:05)")
	showVersion := flag.Bool("version", false, "print version and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "gapwatch - insert visual gap markers into streaming output\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  tail -f /var/log/syslog | gapwatch [flags]\n")
		fmt.Fprintf(os.Stderr, "  kubectl logs -f deploy/app | gapwatch --start-delay 10s --marker '---'\n")
		fmt.Fprintf(os.Stderr, "  journalctl -f | gapwatch --fold 5 --max 20\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("gapwatch %s\n", version)
		os.Exit(0)
	}

	if err := run(os.Stdin, os.Stdout, *startDelay, *marker, *interval, *fold, *max, *timestamp); err != nil {
		if err != io.EOF {
			fmt.Fprintf(os.Stderr, "gapwatch: %v\n", err)
			os.Exit(1)
		}
	}
}

func run(in io.Reader, out io.Writer, startDelay time.Duration, marker string, interval time.Duration, fold int, max int, timestamp bool) error {
	lines := make(chan string)
	errs := make(chan error, 1)

	go func() {
		scanner := bufio.NewScanner(in)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			errs <- err
		} else {
			errs <- io.EOF
		}
		close(lines)
	}()

	// Wait for first line before starting anything
	firstLine, ok := <-lines
	if !ok {
		return <-errs
	}
	fmt.Fprintln(out, firstLine)

	// One-time warm-up: pass lines through, no gap detection
	if startDelay > 0 {
		warmup := time.NewTimer(startDelay)
		warmupDone := false
		for !warmupDone {
			select {
			case line, ok := <-lines:
				if !ok {
					warmup.Stop()
					return <-errs
				}
				fmt.Fprintln(out, line)
			case <-warmup.C:
				warmupDone = true
			}
		}
	}

	silenceTimer := time.NewTimer(interval)
	defer silenceTimer.Stop()

	markerCount := 0
	tickCount := 0

	for {
		select {
		case line, ok := <-lines:
			if !ok {
				return <-errs
			}
			fmt.Fprintln(out, line)
			if !silenceTimer.Stop() {
				select {
				case <-silenceTimer.C:
				default:
				}
			}
			silenceTimer.Reset(interval)
			tickCount = 0

		case <-silenceTimer.C:
			tickCount++
			if fold == 0 || tickCount%fold == 0 {
				if max == 0 || markerCount < max {
					if timestamp {
						fmt.Fprintf(out, "%s %s\n", time.Now().Format("15:04:05"), marker)
					} else {
						fmt.Fprintln(out, marker)
					}
					markerCount++
					if max > 0 && markerCount == max {
						fmt.Fprintln(out, "[gapwatch: max markers reached]")
					}
				}
			}
			silenceTimer.Reset(interval)
		}
	}
}
