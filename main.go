package main // import "moul.io/dl"

import (
	"crypto/tls"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/mholt/archiver"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
	cli "gopkg.in/urfave/cli.v2"
)

func main() {
	app := &cli.App{
		Name: "dl",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "install", Aliases: []string{"i"}},
			&cli.StringFlag{Name: "output", Aliases: []string{"o", "O"}},
			&cli.StringFlag{Name: "unarchive"},
			&cli.BoolFlag{Name: "debug", Aliases: []string{"D"}},
			&cli.BoolFlag{Name: "insecure"},
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
	if c.Bool("install") && c.String("output") == "-" {
		return errors.New(`cannot have --install and --output="-" together`)
	}
	if c.String("archive") != "" && c.String("output") != "" {
		return errors.New(`cannot have --output=... and --unarchive=... together`)
	}
	if c.Bool("debug") {
		log.SetLevel(log.DebugLevel)
	}
	start := time.Now()

	url := c.Args().First()

	unarchive := c.String("unarchive")

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

	if c.Bool("insecure") {
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	downloadLocation := output
	if unarchive != "" {
		tmpDir, err := ioutil.TempDir("", "dl")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)
		downloadLocation = path.Join(tmpDir, path.Base(url))
	}

	var out io.WriteCloser
	switch output {
	case "-":
		out = os.Stdout
	default:
		if err := os.MkdirAll(path.Dir(downloadLocation), 0775); err != nil {
			return err
		}
		var err error
		out, err = os.OpenFile(downloadLocation, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, chmod)
		if err != nil {
			return err
		}
		defer out.Close()
	}

	length, err := io.Copy(out, resp.Body)
	log.
		WithField("location", downloadLocation).
		WithField("length", length).
		WithField("duration", time.Now().Sub(start)).
		Debug("file successfully downloaded")

	if unarchive != "" {
		filesToExtract := map[string]bool{}
		for _, name := range strings.Split(c.String("unarchive"), ",") {
			filesToExtract[name] = false
		}
		err := archiver.Walk(downloadLocation, func(f archiver.File) error {
			if _, found := filesToExtract[f.Name()]; unarchive == "*" || found {
				extractLocation := path.Join(path.Dir(output), f.Name())
				extractFile, err := os.OpenFile(extractLocation, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, chmod)
				if err != nil {
					return err
				}
				log.WithField("location", extractLocation).Debug("extract file from archive")
				defer extractFile.Close()
				if _, err = io.Copy(extractFile, f); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	// ensure chmod is set (when writing over an existing file)
	if err := os.Chmod(downloadLocation, chmod); err != nil {
		return err
	}
	return err
}
