package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func forward(url string, c *gin.Context) error {
	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s/key", url), c.Request.Body)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var responseBody map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&responseBody); err != nil {
		return err
	}

	c.JSON(resp.StatusCode, responseBody)
	return nil
}
