package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	_ "github.com/mattn/go-sqlite3"
	chart "github.com/wcharczuk/go-chart"
	"github.com/wcharczuk/go-chart/util"
	"github.com/y-yagi/configure"
	"github.com/y-yagi/weatherer"
)

const cmd = "weatherer"

type config struct {
	DataBase string `toml:"database"`
}

var cfg config

func init() {
	err := configure.Load(cmd, &cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(cfg.DataBase) == 0 {
		cfg.DataBase = filepath.Join(configure.ConfigDir(cmd), cmd+".db")
		configure.Save(cmd, cfg)
	}
}

func main() {
	os.Exit(run(os.Args, os.Stdout, os.Stderr))
}

func run(args []string, outStream, errStream io.Writer) (exitCode int) {
	var importFile string
	var date string
	var config bool

	exitCode = 0

	flags := flag.NewFlagSet(cmd, flag.ExitOnError)
	flags.SetOutput(errStream)
	flags.StringVar(&importFile, "i", "", "Import file.")
	flags.BoolVar(&config, "c", false, "Edit config.")
	flags.StringVar(&date, "s", "", "Show chart.")
	flags.Parse(args[1:])

	if config {
		editor := os.Getenv("EDITOR")
		if len(editor) == 0 {
			editor = "vim"
		}

		if err := configure.Edit(cmd, editor); err != nil {
			fmt.Fprintf(errStream, "Error: %v\n", err)
			exitCode = 1
			return
		}
		return
	}

	we := weatherer.NewWeatherer(cfg.DataBase)
	err := we.InitDB()
	if err != nil {
		fmt.Fprintf(errStream, "Error: %v\n", err)
		exitCode = 1
		return
	}

	if len(importFile) != 0 {
		if err = we.Import(importFile); err != nil {
			fmt.Fprintf(errStream, "Error: %v\n", err)
			exitCode = 1
			return
		}
		return
	} else if len(date) != 0 {
		if err = drawChart(we, date); err != nil {
			fmt.Fprintf(errStream, "Error: %v\n", err)
			exitCode = 1
			return
		}
		return
	}

	cmd := exec.Command("sqlite3", cfg.DataBase)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err = cmd.Run(); err != nil {
		fmt.Fprintf(errStream, "Error: %v\n", err)
		exitCode = 1
		return
	}
	return
}

func drawChart(we *weatherer.Weatherer, date string) error {
	format := "2006/01/02"
	start, err := time.Parse(format, date)
	if err != nil {
		return err
	}

	end := start.Add(time.Hour * 23)

	weathers, err := we.SelectWeathers(start, end)
	if err != nil {
		return err
	}

	if len(weathers) == 0 {
		return errors.New("no data for the specified date")
	}
	var xvalues []float64
	var yvalues []float64

	for _, weather := range weathers {
		xvalues = append(xvalues, util.Time.ToFloat64(weather.Date))
		yvalues = append(yvalues, weather.Temperature)
	}

	graph := chart.Chart{
		XAxis: chart.XAxis{
			Name:           date,
			NameStyle:      chart.StyleShow(),
			Style:          chart.StyleShow(),
			ValueFormatter: chart.TimeHourValueFormatter,
		},
		YAxis: chart.YAxis{
			Name:      "Temperature",
			NameStyle: chart.StyleShow(),
			Style:     chart.StyleShow(),
			Range:     &chart.ContinuousRange{Min: 0.0, Max: 40.0},
		},
		Series: []chart.Series{
			chart.ContinuousSeries{
				Style: chart.Style{
					Show: true,
				},
				XValues: xvalues,
				YValues: yvalues,
			},
		},
	}

	tmpfile, err := ioutil.TempFile("", "weatherer-")
	if err != nil {
		return nil
	}

	err = graph.Render(chart.PNG, tmpfile)
	if err != nil {
		return nil
	}
	tmpfile.Close()

	defer os.Remove(tmpfile.Name())
	cmd := exec.Command(openCommand(), tmpfile.Name())
	cmd.Start()
	time.Sleep(1 * time.Second) // NOTE: wait for open file

	return nil
}

func openCommand() string {
	command := ""
	os := runtime.GOOS

	if os == "linux" {
		command = "xdg-open"
	} else if os == "darwin" {
		command = "open"
	}

	return command
}
