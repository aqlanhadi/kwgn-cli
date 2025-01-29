package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
    cfgFile string
    rootCmd = &cobra.Command{
        Use:   "kwgn",
        Short: "A brief description of your application",
        Long:  `kwgn is a utility to extract structured data out of your financial statements`,
    }
)

func Execute() {
    err := rootCmd.Execute()
    if err != nil {
        os.Exit(1)
    }
}

func init() {
    cobra.OnInitialize(initConfig)
    // rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "./.kwgn.yaml", "config file (default is $HOME/.kwgn.yaml)")
    // rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func initConfig() {
    if cfgFile != "" {
        viper.SetConfigFile(cfgFile)
        
    } else {
        home, err := os.UserHomeDir()
        cobra.CheckErr(err)
        viper.AddConfigPath(home)
        viper.AddConfigPath(".")
        viper.SetConfigName(".kwgn")
        viper.SetConfigType("yaml")
    }

    viper.AutomaticEnv()

    if err := viper.ReadInConfig(); err == nil {
        // fmt.Println("Using config file:", viper.ConfigFileUsed())
        // fmt.Println("All settings:", viper.AllSettings())
        // fmt.Println(viper.AllSettings())
    } else {
        fmt.Printf("Error reading config file: %v\n", err)
    }
}