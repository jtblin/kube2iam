package cmd

import (
	log "github.com/Sirupsen/logrus"
	"time"
)

const (
	retryTime    time.Duration = time.Duration(1) * time.Minute
	expireWindow time.Duration = time.Duration(10) * time.Minute
)

type refreshers struct {
	// map of role+remoteIP to channel
	channels map[string]chan bool
	iam      *iam
}

func refresher(iam *iam, role, remoteIP string, close chan bool) {
	roleARN := iam.roleARN(role)
	tickerTime := 0 * time.Second
	successTime := iam.ttl - expireWindow

	var ticker *time.Ticker = nil
	for {
		credentials, err := iam.assumeRoleNoCache(roleARN, remoteIP)

		if err != nil {
			log.Errorf("Error refreshing role %s for ip %s: %s", role, remoteIP, err.Error())
			if tickerTime != retryTime {
				tickerTime = retryTime
				if ticker != nil {
					ticker.Stop()
				}
				ticker = time.NewTicker(tickerTime)
			}
		} else {
			iam.cacheCredentials(roleARN, remoteIP, credentials)
			if tickerTime != successTime {
				tickerTime = successTime
				if ticker != nil {
					ticker.Stop()
				}
				ticker = time.NewTicker(tickerTime)
			}
		}
		select {
		case <-ticker.C:
			continue
		case <-close:
			ticker.Stop()
			return
		}
	}
}

func newRefreshers(iam *iam) *refreshers {
	return &refreshers{
		iam:      iam,
		channels: make(map[string]chan bool),
	}
}

// Starts a credentials refresher for the given role and pod ip
func (refreshers *refreshers) startRefresher(role, remoteIP string) {
	quit := make(chan bool)
	go refresher(refreshers.iam, role, remoteIP, quit)
	refreshers.channels[role+remoteIP] = quit
}

// Stops a credentials refresher for the given role and pod ip
func (refreshers *refreshers) stopRefresher(role, remoteIP string) {
	quit := refreshers.channels[role+remoteIP]
	quit <- true
	close(quit)
	delete(refreshers.channels, role+remoteIP)
}
