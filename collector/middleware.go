package collector

import "github.com/gin-gonic/gin"

func ValidateApiKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: implement ApiKey validation
		// check if the request has the Secure Header
		// validate that the value in the header is present in Redis
		// discard request if either not present or incorrect key.
		// this will be useful for multitenancy
		c.Next()
	}
}
