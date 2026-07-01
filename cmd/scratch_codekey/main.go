package main

import (
        "fmt"
        "metapi/aggrsite/platform"
)

func main() {
        const (
                baseURL  = "https://api.123nhh.com"
                username = "jaciloy"
                password = "b3uHguOrR5k3"
        )
        useSystemProxy := false
        opt := &platform.RequestOption{
                UseSystemProxy: &useSystemProxy,
        }
        adapter := platform.GetAdapter("new-api")
        if adapter == nil {
                fmt.Println("ERROR: new-api adapter not found")
                return
        }

        // Step 1: Login
        fmt.Println("=== Step 1: Login ===")
        loginResult, err := adapter.Login(baseURL, username, password, opt)
        if err != nil {
                fmt.Printf("Login error: %v\n", err)
                return
        }
        fmt.Printf("Success: %v\n", loginResult.Success)
        fmt.Printf("Message: %s\n", loginResult.Message)
        fmt.Printf("AccessToken: %s\n", loginResult.AccessToken)
        fmt.Printf("IsCookie: %v\n", platform.IsCookieSessionToken(loginResult.AccessToken))

        if !loginResult.Success {
                fmt.Println("Login failed, stopping.")
                return
        }

        accessToken := loginResult.AccessToken

        // Step 2: Verify token / get balance
        fmt.Println("\n=== Step 2: GetBalance ===")
        balance, err := adapter.GetBalance(baseURL, accessToken, loginResult.PlatformUserID, opt)
        if err != nil {
                fmt.Printf("GetBalance error: %v\n", err)
        } else {
                fmt.Printf("Balance: %.4f, Used: %.4f, Quota: %.4f\n", balance.Balance, balance.Used, balance.Quota)
        }

        // Step 3: Checkin
        fmt.Println("\n=== Step 3: Checkin ===")
        checkinResult, err := adapter.Checkin(baseURL, accessToken, loginResult.PlatformUserID, opt)
        if err != nil {
                fmt.Printf("Checkin error: %v\n", err)
        } else {
                fmt.Printf("Success: %v\n", checkinResult.Success)
                fmt.Printf("Message: %s\n", checkinResult.Message)
                fmt.Printf("Reward: %s\n", checkinResult.Reward)
        }
}
