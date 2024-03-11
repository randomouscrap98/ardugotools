package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/randomouscrap98/ardugotools/arduboy"
	"github.com/urfave/cli/v3"
)

// Quick way to fail on error, since most commands are "doing" something on
// behalf of something else.
func fatalIfErr(subject string, doing string, err error) {
	if err != nil {
		log.Fatalf("%s - Couldn't %s: %s", subject, doing, err)
	}
}

// Scan for arduboys and return json
func scanAction() {
	devices, err := arduboy.GetBasicDevices()
	fatalIfErr("scan", "pull devices", err)
	PrintJson(devices)
}

func connectWithBootloader(device string) (io.ReadWriteCloser, *arduboy.BasicDeviceInfo) {
	sercon, d, err := arduboy.ConnectWithBootloader(device)
	fatalIfErr(device, "connect", err)
	return sercon, d
}

func queryAction(device string) {
	sercon, d := connectWithBootloader(device)
	extdata, err := arduboy.QueryDevice(d, sercon)
	fatalIfErr(device, "query device information", err)
	PrintJson(extdata)
}

func sketchReadAction(device string, filename string) {
	// Read sketch First
	sercon, _ := connectWithBootloader(device)
	sketch, err := arduboy.ReadSketch(sercon)
	fatalIfErr(device, "read sketch", err)
	hash := arduboy.Md5String(sketch)
	if filename == "" {
		filename = fmt.Sprintf("%s_%s.hex", hash, FileSafeDateTime())
	}
	result := make(map[string]interface{})
	result["Filename"] = filename
	result["MD5"] = hash
	PrintJson(result)
}

func main() {
	// Used this example: https://github.com/urfave/cli/blob/main/docs/v3/examples/subcommands.md
	// And this: https://github.com/urfave/cli/blob/main/docs/v3/examples/combining-short-options.md
	cmd := &cli.Command{
		Name:  "ardugotools",
		Usage: "A set of commands for working Arduboy on the command line",
		Description: "This is a reimplementation of the Arduboy Toolset GUI as a command line tool. " +
			"It is designed primarily to be used inside scripts, so some output may not (yet) have nice formatting. " +
			"You can use 'any' in place of any [port] argument to connect to the first arduboy found",
		Version: "0.1.0",
		Authors: []any{"haloopdy"},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "outfile",
				Aliases: []string{"o"},
				Usage:   "Save to `FILE`",
			},
			&cli.StringFlag{
				Name:    "infile",
				Aliases: []string{"i"},
				Usage:   "Read from `FILE`",
			},
		},
		Commands: []*cli.Command{
			{
				Name: "scan",
				//Aliases: []string{"a"},
				Usage: "scan for all connected arduboys and return basic info without querying the device",
				Description: "This command queries your operating system for all serial ports and pulls whatever " +
					"data it can without opening the ports or reading/writing over serial. Different operating systems " +
					"provide different amounts of data. An internal VID/PID table is used to lookup the board; if your device " +
					"isn't showing up, it may not yet exist in the table.",
				ArgsUsage: "",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					scanAction()
					return nil
				},
			},
			{
				Name: "query",
				//Aliases: []string{"a"},
				Usage: "query an individual arduboy for more information (may reboot device)",
				Description: "Connect to the given device, querying as much as possible about it. Certain fields, " +
					"like the version, are exact, while others, like the apparent device and bootloader size, are " +
					"a best guess. This command can help you identify the capabilities of the connected Arduboy.",
				ArgsUsage: "[port]",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					queryAction(cmd.Args().First())
					return nil
				},
			},
			{
				Name:  "sketch",
				Usage: "Commands for working with arduboy sketches",
				Commands: []*cli.Command{
					{
						Name:  "read",
						Usage: "read the sketch as-is from the arduboy as a .hex file",
						Description: "Connect to the given device and read the entire sketch contained within the " +
							"flash memory. A default filename is chosen if none is provided, otherwise you can do -o <filename>. " +
							"Files are written as .hex, which is the universal format tools expect to write back to arduboy",
						ArgsUsage: "[port]",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							sketchReadAction(cmd.Args().First(), cmd.String("outfile"))
							return nil
						},
					},
				},
			},
			//{
			//	Name:    "complete",
			//	Aliases: []string{"c"},
			//	Usage:   "complete a task on the list",
			//	Action: func(ctx context.Context, cmd *cli.Command) error {
			//		fmt.Println("completed task: ", cmd.Args().First())
			//		return nil
			//	},
			//},
			//{
			//	Name:    "template",
			//	Aliases: []string{"t"},
			//	Usage:   "options for task templates",
			//	Commands: []*cli.Command{
			//		{
			//			Name:  "add",
			//			Usage: "add a new template",
			//			Action: func(ctx context.Context, cmd *cli.Command) error {
			//				fmt.Println("new task template: ", cmd.Args().First())
			//				return nil
			//			},
			//		},
			//		{
			//			Name:  "remove",
			//			Usage: "remove an existing template",
			//			Action: func(ctx context.Context, cmd *cli.Command) error {
			//				fmt.Println("removed task template: ", cmd.Args().First())
			//				return nil
			//			},
			//		},
			//	},
			//},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
