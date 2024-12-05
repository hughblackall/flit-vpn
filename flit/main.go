package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"syscall"

	"github.com/digitalocean/godo"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	clientID     = "e6d00a6c53b4f4b63ae0156c8e09c4957caeb382d5b63f8b301f710f9aadcbe6" // Only client ID is needed
	authFileName = "credentials"

	appName = "flit-vpn" // Constant application name
)

var digitalOceanRegions = []string{
	"nyc1\tNew York City, United States",
	"nyc2\tNew York City, United States",
	"nyc3\tNew York City, United States",
	"ams3\tAmsterdam, the Netherlands",
	"sfo2\tSan Francisco, United States",
	"sfo3\tSan Francisco, United States",
	"sgp1\tSingapore",
	"lon1\tLondon, United Kingdom",
	"fra1\tFrankfurt, Germany",
	"tor1\tToronto, Canada",
	"blr1\tBangalore, India",
	"syd1\tSydney, Australia",
}

type credentials struct {
	DigitalOceanToken string
	TailscaleKey      string
}

var (
	creds credentials
)

func main() {
	rootCmd := &cobra.Command{Use: "flit"}

	// CLI Commands
	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with DigitalOcean and Tailscale",
		Run:   login,
	}

	upCmd := &cobra.Command{
		Use:       "up [region]",
		Short:     "Create or update the Flit Tailscale node",
		Args:      cobra.ExactArgs(1),
		ValidArgs: digitalOceanRegions,
		Run:       deployApp,
	}
	downCmd := &cobra.Command{
		Use:   "down",
		Short: "Remove the Flit Tailscale node",
		Run:   destroyApp,
	}

	// TODO: Add status command to check if node is running

	rootCmd.AddCommand(loginCmd, upCmd, downCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func login(cmd *cobra.Command, args []string) {

	// Get token from user input
	fmt.Print("Enter a DigitalOcean personal access token: ")
	input, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.Fatalf("Failed to read DigitalOcean token: %v", err)
	}
	creds.DigitalOceanToken = string(input)

	fmt.Print("\nEnter a Tailscale auth key: ")
	input, err = term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.Fatalf("Failed to read Tailscale auth key: %v", err)
	}
	creds.TailscaleKey = string(input)

	saveToken(creds)
}

// Get a DigitalOcean client, checking for authentication
func getClient() *godo.Client {
	if len(creds.DigitalOceanToken) == 0 {
		if loadToken() != nil {
			fmt.Println("Please log in first by running 'flit login'")
			os.Exit(1)
		}
	}
	return godo.NewFromToken(creds.DigitalOceanToken)
}

// Check for tailscale auth key
func getTailscaleKey() string {
	if len(creds.TailscaleKey) == 0 {
		fmt.Println("Please log in first by running 'flit login'")
		os.Exit(1)
	}
	return creds.TailscaleKey
}

// Deploys or updates the application
// TODO: Create a specific project for the app and encapsulate deployment within
func deployApp(cmd *cobra.Command, args []string) {
	client := getClient()
	ctx := context.TODO()
	region := args[0]

	appSpec := &godo.AppSpec{
		Name:   appName,
		Region: region,
		Alerts: []*godo.AppAlertSpec{
			{Rule: "DEPLOYMENT_FAILED"},
			{Rule: "DOMAIN_FAILED"},
		},
		Workers: []*godo.AppWorkerSpec{
			{
				Name:             "tailscale",
				InstanceCount:    1,
				InstanceSizeSlug: "apps-s-1vcpu-0.5gb",
				Image: &godo.ImageSourceSpec{
					Registry:     "tailscale",
					RegistryType: "DOCKER_HUB",
					Repository:   "tailscale",
					Tag:          "stable",
				},
				Envs: []*godo.AppVariableDefinition{
					{
						Key:   "TS_AUTHKEY",
						Scope: "RUN_AND_BUILD_TIME",
						Type:  "SECRET",
						Value: getTailscaleKey(),
					},
					{
						Key:   "TS_EXTRA_ARGS",
						Scope: "RUN_AND_BUILD_TIME",
						Value: "--advertise-exit-node --advertise-tags=tag:digitalocean-exit-node",
					},
					{
						Key:   "TS_HOSTNAME",
						Scope: "RUN_AND_BUILD_TIME",
						Value: fmt.Sprintf("digitalocean-%s", region),
					},
				},
			},
		},
	}

	app, err := findAppByName(ctx, client)
	if err != nil {
		log.Fatalf("Error checking existing apps: %v", err)
	}

	if app != nil {
		fmt.Println("Tailscale node exists, updating...")
		_, _, err := client.Apps.Update(ctx, app.ID, &godo.AppUpdateRequest{
			Spec: appSpec,
		})
		if err != nil {
			log.Fatalf("Failed to update the node: %v", err)
		}
		fmt.Println("Node updated and redeployed successfully.")
	} else {
		fmt.Println("Creating new Tailscale node...")
		_, _, err := client.Apps.Create(ctx, &godo.AppCreateRequest{Spec: appSpec})
		if err != nil {
			log.Fatalf("Failed to create node: %v", err)
		}
		fmt.Println("Node created successfully.")
	}
}

// Deletes the application
func destroyApp(cmd *cobra.Command, args []string) {
	client := getClient()
	ctx := context.TODO()

	app, err := findAppByName(ctx, client)
	if err != nil {
		log.Fatalf("Error checking existing Flit Tailscale nodes: %v", err)
	}
	if app == nil {
		fmt.Println("No Flit Tailscale nodes exist.")
		return
	}

	_, err = client.Apps.Delete(ctx, app.ID)
	if err != nil {
		log.Fatalf("Failed to delete Tailscale node: %v", err)
	}
	fmt.Println("Tailscale node deleted successfully.")
}

// Find app by name
func findAppByName(ctx context.Context, client *godo.Client) (*godo.App, error) {
	apps, _, err := client.Apps.List(ctx, &godo.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, app := range apps {
		if app.Spec.Name == appName {
			return app, nil
		}
	}
	return nil, nil
}

// Credentials management
func saveToken(creds credentials) {
	data, err := json.Marshal(creds)
	if err != nil {
		log.Fatalf("Failed to save token: %v", err)
	}

	configDir := createConfigDirectory()

	if err := os.WriteFile(filepath.Join(configDir, authFileName), []byte(data), 0600); err != nil {
		log.Fatalf("Failed to write token to file: %v", err)
	}
}

func loadToken() error {
	configDir := buildConfigDirPath()
	data, err := os.ReadFile(filepath.Join(configDir, authFileName))
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &creds)
	if err != nil {
		return err
	}

	return nil

	// TODO: Check if tokens are specified in environment and use those instead
}

// Build config directory based on user's home directory
func buildConfigDirPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	return filepath.Join(homeDir, ".flit-vpn")
}

// Create config directory if it doesn't already exist
func createConfigDirectory() string {
	configDir := buildConfigDirPath()

	if err := os.MkdirAll(configDir, 0755); err != nil {
		log.Fatalf("Failed to create config directory: %v", err)
	}

	return configDir
}
