package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/digitalocean/godo"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

var (
	clientID     = "YOUR_DIGITALOCEAN_CLIENT_ID"
	clientSecret = "YOUR_DIGITALOCEAN_CLIENT_SECRET"
	authFile     = filepath.Join(os.TempDir(), "flit_cli_auth.json")
	oauthConfig  *oauth2.Config
	token        *oauth2.Token

	appName = "flit-vpn" // Constant application name
)

func main() {
	rootCmd := &cobra.Command{Use: "flit"}
	oauthConfig = &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{"read", "write"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://cloud.digitalocean.com/v1/oauth/authorize",
			TokenURL: "https://cloud.digitalocean.com/v1/oauth/token",
		},
	}

	// CLI Commands
	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with DigitalOcean using OAuth2",
		Run:   login,
	}

	upCmd := &cobra.Command{
		Use:   "up [region]",
		Short: "Create or update the deployment",
		Args:  cobra.ExactArgs(1),
		Run:   deployApp,
	}
	downCmd := &cobra.Command{
		Use:   "down",
		Short: "Tear down the deployment",
		Run:   destroyApp,
	}

	// Completion command
	completionCmd := &cobra.Command{
		Use:   "completion [shell]",
		Short: "Generate shell completions for bash, zsh, or fish",
		Args:  cobra.ExactArgs(1),
		Run:   generateCompletion,
	}
	rootCmd.AddCommand(loginCmd, upCmd, downCmd, completionCmd)

	cobra.OnInitialize(setRegionCompletion)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

// Login function to initiate OAuth2 flow
func login(cmd *cobra.Command, args []string) {
	port := getRandomPort()
	redirectURI := fmt.Sprintf("http://localhost:%d/oauth/callback", port)
	oauthConfig.RedirectURL = redirectURI

	url := oauthConfig.AuthCodeURL("state", oauth2.AccessTypeOffline)
	fmt.Println("Opening browser for authorization...")
	if err := exec.Command("xdg-open", url).Start(); err != nil {
		fmt.Printf("Error opening browser: %v\n", err)
		return
	}

	http.HandleFunc("/oauth/callback", handleCallback)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func handleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Authorization code not found", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	tok, err := oauthConfig.Exchange(ctx, code)
	if err != nil {
		http.Error(w, "Failed to exchange token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	token = tok
	saveToken(token)

	fmt.Fprintln(w, "Login successful! You may close this window.")
	fmt.Println("Authentication successful. Token saved.")
}

// Get a DigitalOcean client, checking for authentication
func getClient() *godo.Client {
	if token == nil {
		if !loadToken() {
			fmt.Println("Please log in first by running 'flit login'")
			os.Exit(1)
		}
	}
	tokenSource := oauth2.StaticTokenSource(token)
	oauthClient := oauth2.NewClient(context.Background(), tokenSource)
	return godo.NewClient(oauthClient)
}

// Deploys or updates the application
func deployApp(cmd *cobra.Command, args []string) {
	client := getClient()
	ctx := context.TODO()
	region := args[0]

	app, err := findAppByName(ctx, client)
	if err != nil {
		log.Fatalf("Error checking existing apps: %v", err)
	}

	if app != nil {
		fmt.Println("Application exists, updating...")
		_, _, err := client.Apps.Update(ctx, app.ID, &godo.AppUpdateRequest{
			Spec: &godo.AppSpec{
				Name:   appName,
				Region: region,
			},
		})
		if err != nil {
			log.Fatalf("Failed to update app: %v", err)
		}
		fmt.Println("App updated and redeployed successfully.")
	} else {
		fmt.Println("Creating new application...")
		appSpec := &godo.AppSpec{
			Name:   appName,
			Region: region,
		}
		_, _, err := client.Apps.Create(ctx, appSpec)
		if err != nil {
			log.Fatalf("Failed to create app: %v", err)
		}
		fmt.Println("App created and deployed successfully.")
	}
}

// Deletes the application
func destroyApp(cmd *cobra.Command, args []string) {
	client := getClient()
	ctx := context.TODO()

	app, err := findAppByName(ctx, client)
	if err != nil {
		log.Fatalf("Error checking existing apps: %v", err)
	}
	if app == nil {
		fmt.Println("Application does not exist.")
		return
	}

	_, err = client.Apps.Delete(ctx, app.ID)
	if err != nil {
		log.Fatalf("Failed to delete app: %v", err)
	}
	fmt.Println("App deleted successfully.")
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

// Generate shell completion script for a specific shell and print to stdout
func generateCompletion(cmd *cobra.Command, args []string) {
	shell := args[0]
	switch shell {
	case "bash":
		cmd.Root().GenBashCompletion(os.Stdout)
	case "zsh":
		cmd.Root().GenZshCompletion(os.Stdout)
	case "fish":
		cmd.Root().GenFishCompletion(os.Stdout, true)
	default:
		fmt.Printf("Unsupported shell: %s\n", shell)
	}
}

// Sets dynamic completion for the region argument with DigitalOcean regions
func setRegionCompletion() {
	upCmd := &cobra.Command{}
	client := getClient()
	ctx := context.TODO()
	regions, err := getAllRegions(ctx, client)
	if err != nil {
		log.Fatalf("Failed to get regions: %v", err)
	}
	upCmd.RegisterFlagCompletionFunc("region", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return regions, cobra.ShellCompDirectiveDefault
	})
}

// Fetch all DigitalOcean regions for completion
func getAllRegions(ctx context.Context, client *godo.Client) ([]string, error) {
	regions, _, err := client.Regions.List(ctx, &godo.ListOptions{})
	if err != nil {
		return nil, err
	}
	var regionNames []string
	for _, region := range regions {
		regionNames = append(regionNames, region.Slug)
	}
	return regionNames, nil
}

// Generate a random port for OAuth callback
func getRandomPort() int {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal("Error finding open port:", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

// Token management
func saveToken(token *oauth2.Token) {
	data, err := json.Marshal(token)
	if err != nil {
		log.Fatalf("Failed to save token: %v", err)
	}
	if err := os.WriteFile(authFile, data, 0600); err != nil {
		log.Fatalf("Failed to write token to file: %v", err)
	}
}

func loadToken() bool {
	data, err := os.ReadFile(authFile)
	if err != nil {
		return false
	}
	if err := json.Unmarshal(data, &token); err != nil {
		return false
	}
	return true
}
