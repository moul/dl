package main // import "moul.io/dl"

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	log "github.com/sirupsen/logrus"
	cli "gopkg.in/urfave/cli.v2"
)

func main() {
	app := &cli.App{
		Name: "dl",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "install", Aliases: []string{"i"}},
			&cli.StringFlag{Name: "output", Aliases: []string{"o", "O"}},
			// &cli.StringFlag{Name: "chmod"},
			&cli.BoolFlag{Name: "debug", Aliases: []string{"D"}},
			&cli.StringFlag{Name: "chmod", Aliases: []string{"c"}, Value: "664"},
		},
		Action: dl,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatalf("fatal error: %v", err)
	}
}

func dl(c *cli.Context) error {
	if c.NArg() != 1 {
		return cli.Exit("usage: dl [OPTS...] URL", 1)
	}
	if c.Bool("debug") {
		log.SetLevel(log.DebugLevel)
	}
	start := time.Now()

	url := c.Args().First()

	output := c.String("output")
	if output == "" {
		output = path.Base(url)
	}
	chmodInt, err := strconv.ParseInt(c.String("chmod"), 8, 32)
	if err != nil {
		return err
	}
	chmod := os.FileMode(chmodInt)
	if c.Bool("install") {
		if output == "-" {
			return errors.New(`cannot have --install and --output="-" together`)
		}
		// FIXME: support windows
		availablePaths := strings.Split(os.Getenv("PATH"), ":")
		log.WithField("available-paths", availablePaths).Debug("looking up for a writable directory in $PATH")
		for _, currentPath := range availablePaths {
			if unix.Access(currentPath, unix.W_OK) == nil {
				log.WithField("writable-path", currentPath).Debug("selected the install directory")
				output = path.Join(currentPath, output)
				break
			}
		}
		chmod |= 0111
	}
	log.WithField("output", output).Debug("selecting output")
	log.WithField("url", url).Debug("starting download")
	log.WithField("chmod", chmod).Debug("file permissions")

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var out io.WriteCloser
	switch output {
	case "-":
		out = os.Stdout
	default:
		if err := os.MkdirAll(path.Dir(output), 0775); err != nil {
			return err
		}
		var err error
		out, err = os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, chmod)
		if err != nil {
			return err
		}
		defer out.Close()
	}

	// ensure chmod is set (when writing over an existing file)
	if err := os.Chmod(output, chmod); err != nil {
		return err
	}
	length, err := io.Copy(out, resp.Body)
	log.
		WithField("output", output).
		WithField("length", length).
		WithField("duration", time.Now().Sub(start)).
		Info("file successfully downloaded")
	return err
}
