package pluginrouter

import (
	"argocd-secrets-plugin/internal/pluginkube"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"k8s.io/client-go/kubernetes"
)

// RequestPayload represents the input parameters structure in the request body
type RequestPayload struct {
	ApplicationSetName string `json:"applicationSetName"`
	Input              struct {
		Parameters struct {
			SecretName      string                 `json:"secretName"`
			NamespaceName   string                 `json:"namespaceName"`
			ConvertToGoVars interface{}            `json:"convertToGoVars,omitempty"`
			Other           map[string]interface{} `json:"-"`
		} `json:"parameters"`
	} `json:"input"`
}

// ConvertToBoolean converts a string or boolean to a boolean value
func ConvertToBoolean(value interface{}) bool {
	switch v := value.(type) {
	case string:
		return strings.ToLower(v) == "true"
	case bool:
		return v
	default:
		return false
	}
}

// ResponsePayload represents the structure of the response
type ResponsePayload struct {
	Output struct {
		Parameters []map[string]interface{} `json:"parameters"`
	} `json:"output"`
}

// Middleware to check the Authorization header
func AuthMiddleware(authToken string) gin.HandlerFunc {
	return func(c *gin.Context) {

		// Extract the Authorization header from the request
		authHeader := c.GetHeader("Authorization")

		// Check if the Authorization header contains the correct Bearer token
		if !strings.HasPrefix(authHeader, "Bearer ") || strings.TrimPrefix(authHeader, "Bearer ") != authToken {
			c.JSON(http.StatusForbidden, gin.H{"error": "Invalid or missing token"})
			c.Abort()
			return
		}

		// Continue processing if the token is valid
		c.Next()
	}
}

func RouterSet(clientset *kubernetes.Clientset) {

	var authToken string
	var namespace string

	if authTokenEnv := os.Getenv("ARGOCD_PLUGIN_TOKEN"); authTokenEnv != "" {
		authToken = authTokenEnv
	} else {
		log.Fatalf("ARGOCD_PLUGIN_TOKEN environment variable is not set, exitting")

	}

	// Create a new Gin router
	router := gin.Default()

	// Apply the AuthMiddleware to the route
	router.POST("/api/v1/getparams.execute", AuthMiddleware(authToken), func(c *gin.Context) {
		var requestPayload RequestPayload

		// Parse JSON body into requestPayload
		if err := c.ShouldBindJSON(&requestPayload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
			return
		}

		convertToGoVars := ConvertToBoolean(requestPayload.Input.Parameters.ConvertToGoVars)

		log.Printf("Received request for applicationSetName: %s to read from secret %s", requestPayload.ApplicationSetName, requestPayload.Input.Parameters.SecretName)

		if convertToGoVars {
			log.Printf("ConvertToGoVars set to True, secret key names will be changed to Golang format")
		}

		secretName := requestPayload.Input.Parameters.SecretName

		// Parsing Namespace

		if requestPayload.Input.Parameters.NamespaceName != "" {
			namespace = requestPayload.Input.Parameters.NamespaceName

		} else {
			namespace = "argocd"
		}

		// Read the secret
		secret, err := pluginkube.ReadSecret(pluginkube.KubeClientSet(), namespace, secretName, convertToGoVars)
		if err != nil {
			fmt.Printf("failed to get secret: %v", err)

		}

		// Prepare response payload
		var responsePayload ResponsePayload
		// Echo back the input parameters as a list of one object map
		// Include secretName and any other parameters
		parameters := map[string]interface{}{
			"secretName": requestPayload.Input.Parameters.SecretName,
		}

		// Merge additional parameters from the request

		for key, value := range secret {
			fmt.Printf("Value read for key: %s\n", key)
			parameters[key] = value
		}

		responsePayload.Output.Parameters = append(responsePayload.Output.Parameters, parameters)

		// Respond with the output payload
		c.JSON(http.StatusOK, responsePayload)

	})

	router.Run(":8080")

}
