package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"

	dep "github.com/hashicorp/consul-template/dependency"
	"github.com/hashicorp/consul-template/watch"
)

func main() {
	// make the consul-template library logs not show up
	log.SetOutput(ioutil.Discard)

	// setup Vault API client
	clients := dep.NewClientSet()
	clients.CreateVaultClient(&dep.CreateVaultClientInput{
		Address:     "http://127.0.0.1:8200",
		Namespace:   "",
		Token:       "hvs.Ic25dAXwQNgEFzINCVlzgySC",
		UnwrapToken: false,
		SSLEnabled:  false,
		SSLVerify:   false,
		SSLCert:     "",
		SSLKey:      "",
		SSLCACert:   "",
		SSLCAPath:   "",
		ServerName:  "",
	})

	// create new watcher with the Vault API client
	w, err := watch.NewWatcher(&watch.NewWatcherInput{
		Clients: clients,
	})
	if err != nil {
		fmt.Printf("err: %w\n", err)
	}

	// setup a dependency to watch the Vault secret "secret/hello"
	vrq, err := dep.NewVaultReadQuery("secret/hello")
	if err != nil {
		fmt.Println("tried to vault read query")
		fmt.Printf("err: %w\n", err)
	}
	// fetch the initial secret, and store it in the secretmap, so we can compare it during the polling
	i, _, err := vrq.Fetch(clients, &dep.QueryOptions{})
	secretmap := map[string]string{}
	secret := i.(*dep.Secret)
	secretmap["secret/hello"] = secret.Data["data"].(map[string]interface{})["foo"].(string)

	// add the dependency to the watcher
	w.Add(vrq)

	datach := w.DataCh()
	errch := w.ErrCh()

	// this goroutine watches the poller, and compares it to what's in the secretmap to indicate if there's a change
	go func() {
		for {
			select {
			// check if there's a change to the secret and print
			case got := <-datach:
				vaultsecret := got.Data().(*dep.Secret)
				if secretmap["secret/hello"] != vaultsecret.Data["data"].(map[string]interface{})["foo"].(string) {
					fmt.Printf("Secret %s has been changed", "secret/hello")
				}
				//fmt.Println(got.DataAndLastIndex())
				secretmap["secret/hello"] = vaultsecret.Data["data"].(map[string]interface{})["foo"].(string)
			case got := <-errch:
				fmt.Println("got an error for the dependency")
				fmt.Printf("err %w\n", got)
			}
		}
	}()

	// make the main run until interrupt
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	<-c

}
