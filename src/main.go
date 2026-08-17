package main

import (
	"log"
	"net/http"
	"os"

	"github.com/OpenFactorioServerManager/factorio-server-manager/src/api"
	"github.com/OpenFactorioServerManager/factorio-server-manager/src/bootstrap"
	"github.com/OpenFactorioServerManager/factorio-server-manager/src/factorio"
)

func main() {
	// get the all configs based on the flags
	config := bootstrap.NewConfig(os.Args[1:])

	// setup the required files for the mods
	factorio.ModStartUp()

	// Initialize Factorio Server struct
	err := factorio.NewFactorioServer()
	if err != nil {
		log.Printf("Error occurred during Server initialization: %v\n", err)
		return
	}
	factorio.StartSaveBackupScheduler()
	factorio.StartLifecycleScheduler()

	// Initialize authentication system
	api.SetupAuth()

	// Initialize HTTP router -- also initializes websocket
	router := api.NewRouter()

	log.Printf("Starting server on: %s:%s", config.ServerIP, config.ServerPort)
	log.Fatal(http.ListenAndServe(config.ServerIP+":"+config.ServerPort, router))

}
