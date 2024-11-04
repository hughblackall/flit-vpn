package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"github.com/digitalocean/godo"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

const (
	clientID = "e6d00a6c53b4f4b63ae0156c8e09c4957caeb382d5b63f8b301f710f9aadcbe6" // Only client ID is needed
	authFile = "flit_cli_auth.json"

	appName = "flit-vpn" // Constant application name
)

// OAuth2 variables
var (
	oauthConfig   *oauth2.Config
	token         *oauth2.Token
	state         string
	codeVerifier  string // For PKCE
	codeChallenge string // Hashed version of codeVerifier
)

func main() {
	rootCmd := &cobra.Command{Use: "flit"}
	oauthConfig = &oauth2.Config{
		ClientID: clientID,
		Scopes:   []string{"read", "write"},
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

	// // Completion command
	// completionCmd := &cobra.Command{
	// 	Use:   "completion [shell]",
	// 	Short: "Generate shell completions for bash, zsh, or fish",
	// 	Args:  cobra.ExactArgs(1),
	// 	Run:   generateCompletion,
	// }

	rootCmd.AddCommand(loginCmd, upCmd, downCmd) //, completionCmd)

	// cobra.OnInitialize(setRegionCompletion)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

// Generate a random code verifier
func generateCodeVerifier() string { // , error
	// bytes := make([]byte, 32)
	// if _, err := rand.Read(bytes); err != nil {
	// 	return "", err
	// }
	// return base64.RawURLEncoding.EncodeToString(bytes), nil
	return oauth2.GenerateVerifier()
}

// Generate code challenge from the code verifier
func generateCodeChallenge(codeVerifier string) string {
	// hasher := sha256.New()
	// hasher.Write([]byte(codeVerifier))
	// return base64.RawURLEncoding.EncodeToString(hasher.Sum(nil))
	return oauth2.S256ChallengeFromVerifier(codeVerifier)
}

func generateSecureRandomString(length int) string {
	characters := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	b := make([]byte, length)
	rand.Read(b)

	// Convert bytes to printable string
	for i := 0; i < len(b); i++ {
		b[i] = characters[b[i]%byte(len(characters))]
	}

	return string(b)
}

// Login function for PKCE authentication
func login(cmd *cobra.Command, args []string) {
	var err error

	// Generate code verifier and code challenge
	codeVerifier = generateCodeVerifier()
	// if err != nil {
	// 	log.Fatalf("Failed to generate code verifier: %v", err)
	// }
	codeChallenge = generateCodeChallenge(codeVerifier)
	state = generateSecureRandomString(32)

	// Dynamic port allocation for redirect URI
	port := 8080 // getRandomPort()
	oauthConfig.RedirectURL = fmt.Sprintf("http://localhost:%d/oauth/callback", port)

	// Authorization URL
	// url := oauthConfig.AuthCodeURL(codeChallenge, oauth2.AccessTypeOffline)
	url := oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(codeVerifier), oauth2.SetAuthURLParam("response_type", "token"))
	fmt.Println("Opening browser for authorization...")
	err = openBrowser(url)
	if err != nil {
		log.Fatalf("Error opening browser: %v", err)
	}

	// Handle callback
	http.HandleFunc("/oauth/callback", handleCallback)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

// Handle OAuth2 callback to exchange authorization code for tokens
func handleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Authorization code not found", http.StatusBadRequest)
		return
	}

	// TODO: Check state query parameter and verify it matches the state parameter sent in previous request

	// Exchange the authorization code for an access token
	ctx := context.Background()
	token, err := oauthConfig.Exchange(ctx, code, oauth2.VerifierOption(codeVerifier)) // oauth2.SetAuthURLParam("code_verifier", codeVerifier),
	if err != nil {
		http.Error(w, "Failed to exchange token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	saveToken(token)

	fmt.Fprintln(w, "Login successful! You may close this window.")
	fmt.Println("Authentication successful. Token saved.")
}

func openBrowser(url string) error {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}

	return err
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
	appSpec := &godo.AppSpec{
		Name:   appName,
		Region: region,
	}

	app, err := findAppByName(ctx, client)
	if err != nil {
		log.Fatalf("Error checking existing apps: %v", err)
	}

	if app != nil {
		fmt.Println("Application exists, updating...")
		_, _, err := client.Apps.Update(ctx, app.ID, &godo.AppUpdateRequest{
			Spec: appSpec,
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
		_, _, err := client.Apps.Create(ctx, &godo.AppCreateRequest{Spec: appSpec})
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
