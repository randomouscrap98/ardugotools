package main

import (
	"context"
	"log"
	"os"

	"github.com/randomouscrap98/ardugotools/arduboy"
	"github.com/urfave/cli/v3"
)

// Scan for arduboys and return json
func scanAction() {
	devices, err := arduboy.GetBasicDevices()
	if err != nil {
		log.Fatalln("Couldn't pull devices: ", err)
	}
	PrintJson(devices)
}

func queryAction(device string) {
	sercon, d, err := arduboy.ConnectWithBootloader(device)
	if err != nil {
		log.Fatalf("Couldn't connect to '%s': %s", device, err)
	}
	var extdata *arduboy.ExtendedDeviceInfo
	extdata, err = arduboy.QueryDevice(d, sercon)
	PrintJson(extdata)
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
