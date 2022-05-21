package main

import (
	"emmApi/models"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type CheckServiceResponse struct {
	HasVRCPlus bool `json:"has_vrc_plus"`
}

var (
	mutex        = &sync.Mutex{}
	lastChecked  = time.Now()
	usersToCheck = make([]string, 0)
)

func InitCheckService() {
	if !ServiceConfig.CheckService.CheckEnabled {
		return
	}

	ticker := time.NewTicker(15 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				if lastChecked.Unix() < time.Now().Add(-10*time.Second).Unix() {
					lastChecked = time.Now()
					mutex.Lock()
					if len(usersToCheck) == 0 {
						mutex.Unlock()
						continue
					}

					userId, x := usersToCheck[0], usersToCheck[1:]
					usersToCheck = x
					mutex.Unlock()

					fmt.Printf("Checking user... %s\n", userId)

					resp, err := http.Get(fmt.Sprintf(ServiceConfig.CheckService.CheckUrl, userId))

					if err != nil {
						fmt.Printf("Error checking user: %s\n", err)
						QueueUserCheck(userId)
						continue
					}

					if resp.StatusCode != http.StatusOK {
						fmt.Printf("Error checking user: %s\n", resp.Status)
						continue
					}

					serviceResponse := CheckServiceResponse{}
					err = json.NewDecoder(resp.Body).Decode(&serviceResponse)

					if err != nil {
						fmt.Printf("Error checking user: %s\n", err)
						continue
					}

					var u models.User
					tx := DatabaseConnection.Where("user_id = ?", userId).First(&u)

					u.HasVRCPlus = serviceResponse.HasVRCPlus
					u.LastVRCPlusCheck = time.Now()

					DatabaseConnection.Save(&u)

					if tx.Error != nil {
						fmt.Printf("Error checking user: %s\n", tx.Error)
						continue
					}
				}
			}
		}
	}()
}

func IsExpired(user *models.User) bool {
	if !ServiceConfig.CheckService.CheckEnabled {
		return false
	}

	if user.HasVRCPlus && user.LastVRCPlusCheck.Unix() < time.Now().Add(-168*time.Hour).Unix() {
		return true
	}

	if !user.HasVRCPlus && user.LastVRCPlusCheck.Unix() < time.Now().Add(-48*time.Hour).Unix() {
		return true
	}

	return false
}

func QueueUserCheck(userId string) {
	mutex.Lock()
	usersToCheck = append(usersToCheck, userId)
	mutex.Unlock()
}
