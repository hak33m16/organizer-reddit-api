package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/go-querystring/query"
	"github.com/joho/godotenv"
)

type Response struct {
	Status  string      `json:"status"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

type TokenResponse struct {
	Token string `json:"token"`
}

type TokenRequest struct {
	Code string `json:"code"`
}

type RedditTokenRequest struct {
	GrantType   string `url:"grant_type"`
	Code        string `url:"code"`
	RedirectURI string `url:"redirect_uri"`
}

type RedditTokenResponse struct {
	AccessToken string `json:"access_token"`
}

func getTokenReddit(code string) (string, error) {
	request := &RedditTokenRequest{
		GrantType:   "authorization_code",
		Code:        code,
		RedirectURI: "localhost:3000/authorization",
	}

	requestDataValues, err := query.Values(request)

	req, err := http.NewRequest("POST", "https://www.reddit.com/api/v1/access_token", strings.NewReader(requestDataValues.Encode()))
	if err != nil {
		return "", fmt.Errorf("Error when creating POST request for reddit: %w", err)
	}
	req.SetBasicAuth(os.Getenv("REDDIT_APPLICATION_ID"), os.Getenv("REDDIT_APPLICATION_SECRET"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Reddit Locker Server")

	reqBody, _ := httputil.DumpRequestOut(req, true)
	log.Println(string(reqBody))

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Error when requesting access token from reddit: %w", err)
	}

	body, _ := httputil.DumpResponse(res, true)
	log.Println(string(body))

	var resData RedditTokenResponse
	if err := json.NewDecoder(res.Body).Decode(&resData); err != nil {
		return "", fmt.Errorf("Failed to parse reddit access token body: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		return "", fmt.Errorf("Request for token had an error (status %v): %v", res.StatusCode, body)
	}

	if resData.AccessToken != "" {
		return resData.AccessToken, nil
	}

	return "", fmt.Errorf("Request for token was unsuccessful")
}

func postToken(context *gin.Context) {
	var tokenRequest TokenRequest

	if err := context.BindJSON(&tokenRequest); err != nil {
		log.Printf("Encountered error when trying to bind token request to JSON: %v", err)
		context.AbortWithStatusJSON(400, &Response{
			Status:  "error",
			Message: "Malformed request, please try again",
		})

		return
	}

	token, err := getTokenReddit(tokenRequest.Code)
	if err != nil {
		log.Printf("Encountered error when trying to retrieve reddit token: %v", err)
		context.AbortWithStatusJSON(500, &Response{
			Status:  "error",
			Message: "Unexpected error when authenticating with Reddit, please try again",
		})
		return
	}

	context.JSON(http.StatusOK, &Response{
		Status:  "success",
		Message: "",
		Data: &TokenResponse{
			Token: token,
		},
	})
}

func getHealth(context *gin.Context) {
	context.Status(http.StatusOK)
}

func setGinMode() {
	defaultMode := "release"
	ginModeEnv := os.Getenv("GIN_MODE")

	if ginModeEnv != "" {
		gin.SetMode(ginModeEnv)
	} else {
		gin.SetMode(defaultMode)
	}
}

func verifyCredentialsSet() {
	if os.Getenv("REDDIT_APPLICATION_ID") == "" || os.Getenv("REDDIT_APPLICATION_SECRET") == "" {
		log.Fatalf("Both the Reddit application ID and secret must be set")
	}
}

func main() {
	log.Println("Server starting...")

	if os.Getenv("ENVIRONMENT") == "dev" {
		if err := godotenv.Load(".env"); err != nil {
			log.Fatalf("Error loading .env file")
		}
	}
	verifyCredentialsSet()

	setGinMode()
	router := gin.Default()
	// https://pkg.go.dev/github.com/gin-gonic/gin#section-readme
	router.SetTrustedProxies(nil)

	corsConfig := cors.DefaultConfig()
	// TODO: Parameterize this for production vs. localdev
	corsConfig.AllowOrigins = []string{"http://localhost:3000"}
	corsConfig.AddAllowMethods("POST")
	router.Use(cors.New(corsConfig))

	router.GET("/health", getHealth)
	router.POST("/token", postToken)

	router.Run("localhost:8080")
}
