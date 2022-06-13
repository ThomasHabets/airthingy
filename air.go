package main

/**
*
Example commands:
   air list | jq -r '.devices[] | [.id,.location.name,.segment.name] | @tsv'
   12345 Home Office
   12346 Home Bedroom
   12347 Home Kitchen

   air dev  12345 12347 | jq -r '[.id,.location.name,.segment.name] | @tsv'
   12345 Home Office
   12347 Home Kitchen

   air latest  12345 12347 | jq -r '[.data|.time,.co2]|@tsv'  | \
       while read TIME CO2; do echo "$(date "-d@${TIME?}")" "${CO2?}";done
   Fri Jun  3 10:57:01 BST 2022 766
   Fri Jun  3 11:42:05 BST 2022 762


*/

import (
	"context"
	"net/http"
	"fmt"
	"flag"
	"io"
	"os"

	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2/clientcredentials"
)

var (
	// TODO: move to envs.
	clientID = flag.String("client_id", "", "OAuth client ID")
	clientSecret = flag.String("client_secret", "", "OAuth client secret")
)

func handleList(ctx context.Context, client *http.Client, args []string) error {
	if len(args) != 0 {
		log.Fatalf("Extra args on command line: %q", args)
	}
	res, err := client.Get("https://ext-api.airthings.com/v1/devices")
	if err != nil {
		return fmt.Errorf("failed to list devices: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("status not OK: %v", res.Status)
	}
	if _, err := io.Copy(os.Stdout, res.Body); err != nil {
		return err
	}
	fmt.Println()
	return nil
}

func handleDevice(ctx context.Context, client *http.Client, ids []string) error{
	for _, id := range ids {
		res, err := client.Get("https://ext-api.airthings.com/v1/devices/" + id)
		if err != nil {
			return fmt.Errorf("failed to get device %q: %v", id, err)
		}
		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("status not OK for device %q: %v", id, res.Status)
		}
		if _, err := io.Copy(os.Stdout, res.Body);err != nil {
			return fmt.Errorf("failed to get device data for device id %q: %v",id, err)
		}
		fmt.Println()
	}
	return nil
}

func handleLatest(ctx context.Context, client *http.Client, ids []string) error{
	for _, id := range ids {
		res, err := client.Get("https://ext-api.airthings.com/v1/devices/" + id + "/latest-samples")
		if err != nil {
			return fmt.Errorf("failed to get latest samples for device %q: %v", id, err)
		}
		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("status not OK for device %q: %v", id, res.Status)
		}
		if _, err := io.Copy(os.Stdout, res.Body);err != nil {
			return fmt.Errorf("failed to get latest samples for device id %q: %v",id, err)
		}
		fmt.Println()
	}
	return nil
}

func main() {
	flag.Parse()

	ctx := context.Background()

	conf := clientcredentials.Config{
		ClientID:     *clientID,
		ClientSecret: *clientSecret,
		TokenURL:     "https://accounts-api.airthings.com/v1/token",
		Scopes:       []string{"read:device:current_values"},
	}
	client := conf.Client(ctx)

	h, ok := map[string]func(context.Context, *http.Client, []string)error{
		"list": handleList,
		"dev": handleDevice,
		"latest": handleLatest,
	}[flag.Arg(0)]
	if !ok {
		log.Fatalf("Invalid command %q", flag.Arg(0))
	}
	if err := h(ctx, client, flag.Args()[1:]);err!=nil {
		log.Fatal(err)
	}
}
