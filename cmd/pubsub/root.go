package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/zacksfF/PubSubGo/version"

	log "github.com/sirupsen/logrus"
)

var ConfigFile string

// RootCMD represent the base command called without any subcommand
var RootCMD = &cobra.Command{
	Use:     "PubSubGo",
	Version: version.FullVersion(),
	Short:   "Command-line client for PubSubGo",
	Long: `This is the command-line client for the msgbus daemon PubSubGo
	
This lets you publish, subscribe and pulll messages from a running PubSubGo
instance. This is the refrence implementation of using the PubSubgo client 
library for publishing and subscribing to topics`,

	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		//Set logging level
		if viper.GetBool("debug") {
			log.SetLevel(log.DebugLevel)
		} else {
			log.SetLevel(log.InfoLevel)
		}
	},
}

// Execute adds all child commands to the root command
// and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd;
func Execute() {
	if err := RootCMD.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	RootCMD.PersistentFlags().StringVar(
		&ConfigFile, "config", "",
		"config file (default is $HOME/.msgbus.yaml)",
	)

	RootCMD.PersistentFlags().BoolP(
		"debug", "d", false,
		"Enable debug logging",
	)

	RootCMD.PersistentFlags().StringP(
		"uri", "u", "http://localhost:8000",
		"URI to connect to PubSubGo",
	)

	viper.BindPFlag("uri", RootCMD.PersistentFlags().Lookup("uri"))
	viper.SetDefault("uri", "http://localhost:8000/")

	viper.BindPFlag("debug", RootCMD.PersistentFlags().Lookup("debug"))
	viper.SetDefault("debug", false)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if ConfigFile != "" {
		//USe config file the flag.
		viper.SetConfigFile(ConfigFile)
	} else {
		///Find home directory
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		viper.AddConfigPath(home)
		viper.SetConfigName(".pubsubgo.yaml")
	}
	// from the environment
	viper.SetEnvPrefix("PubSubGo")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
