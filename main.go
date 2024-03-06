package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/randomouscrap98/ardugotools/arduboy"
	"github.com/urfave/cli/v3"
)

func scanAction() {
	devices, err := arduboy.GetBasicDevices()
	if err != nil {
		log.Fatalln("Couldn't pull devices: ", err)
	}
	rawjson, err := json.Marshal(devices)
	if err != nil {
		log.Fatalln("Couldn't serialize json: ", err)
	}
	fmt.Println(string(rawjson))
}

func main() {

	cmd := &cli.Command{
		Commands: []*cli.Command{
			{
				Name: "scan",
				//Aliases: []string{"a"},
				Usage: "scan for arduboys and return basic info without connecting",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					scanAction()
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

	//fmt.Println("Done")

	//fmt.Println("Wow?")
	//arduboy.About()
	//arduboy.ListDevices()
	//arduboy.ListDetailedDevices()
}
