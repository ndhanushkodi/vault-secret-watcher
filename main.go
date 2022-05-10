package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hashicorp/hcat"
	"github.com/hashicorp/hcat/tfunc"
)

func main() {
	// make the consul-template library logs not show up
	log.SetOutput(ioutil.Discard)

	clients := hcat.NewClientSet()
	defer clients.Stop()

	clients.AddVault(hcat.VaultInput{
		Address: "http://127.0.0.1:8200",
		Token:   "a_token",
	})

	// create new watcher with the Vault API client
	w := hcat.NewWatcher(hcat.WatcherInput{
		Clients: clients,
		Cache:   hcat.NewStore(),
	})
	tmpl := hcat.NewTemplate(hcat.TemplateInput{
		Name:         "secret/hello",
		Contents:     `{{- with secret "secret/hello" -}}{{- .Data.data.foo -}}{{- end -}}`,
		FuncMapMerge: tfunc.VaultV0(),
	})
	err := w.Register(tmpl)
	if err != nil {
		panic(err)
	}

	rsv := hcat.NewResolver()
	secretmap := map[string]string{}
	go func() {
		for {
			res, err := rsv.Run(tmpl, w)
			if err != nil {
				panic(err)
			}
			if res.Complete {
				vaultsecret := string(res.Contents)
				if secretmap["secret/hello"] != vaultsecret {
					fmt.Printf("Secret '%s' has been changed to '%s'.\n",
						"secret/hello", vaultsecret)
				}
				secretmap["secret/hello"] = vaultsecret
			}
			ctx, cancel := context.WithTimeout(
				context.Background(), time.Second*3)
			err = w.Wait(ctx)
			cancel()
			if err != nil {
				panic(err)
			}
		}
	}()

	// make the main run until interrupt
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}
