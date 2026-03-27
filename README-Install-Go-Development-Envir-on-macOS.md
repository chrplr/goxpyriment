# 🍎 Setting Up Your Go Development Environment (macOS)


If [brew.sh](https://brew.sh/) is installed on your computer, you can simply install Git and Go with the commands:

```
brew install git
brew install go
```

Otherwise, this guide will walk you through setting up **Git**, **Go**, and **Visual Studio Code** on your Mac. 


## 1. Install Apple Command Line Tools
Before installing anything else, macOS needs its basic "developer foundation."

1.  Open your **Terminal** (Press `Cmd + Space` and type "Terminal").
2.  Type the following command and press Enter:
    ```bash
    xcode-select --install
    ```
3.  A popup will appear asking if you want to install the tools. Click **Install** and agree to the terms. This provides the basic framework for Git and other coding tools.

## 2. Install Git
While macOS comes with a version of Git, it is often outdated. It is best to have the official version.

1.  **Download:** Go to [git-scm.com/download/mac](https://git-scm.com/download/mac).
2.  **Installer:** Use the **"binary installer"** (the installer package).
3.  **Verify:** In your Terminal, type:
    ```bash
    git --version
    ```

## 3. Install Go (Golang)
1.  **Download:** Visit [go.dev/dl](https://go.dev/dl/) and select the **macOS** installer (`.pkg`).
    * **Note:** If you have an Apple Silicon chip (M1, M2, M3, etc.), choose the **ARM64** version. For older Intel Macs, choose **x86-64**.
2.  **Install:** Open the package and follow the installation wizard.
3.  **Verify:** Open a **new** Terminal window and type:
    ```bash
    go version
    ```

## 4. Recommended Editor: Visual Studio Code (VS Code)
VS Code is the favorite choice for macOS users because it is lightweight and integrates perfectly with Mac's file system.

1.  **Download:** Get it at [code.visualstudio.com](https://code.visualstudio.com/).
2.  **Install:** It will download as a `.zip`. Unzip it, then **drag the Visual Studio Code app into your "Applications" folder**.
3.  **Setup the "code" command:**
    * Open VS Code.
    * Press `Cmd + Shift + P`.
    * Type `shell command` and select **"Shell Command: Install 'code' command in PATH"**. This lets you open folders from the terminal by typing `code .`.
4.  **Install Go Extension:**
    * Click the **Extensions** icon (four squares) on the left.
    * Search for **"Go"** and install the one by the Go Team at Google.

---

## 🏗️ Your First Program (The "Hello World" Test)

1.  **Create a folder:** In your Terminal, type:
    ```bash
    mkdir ~/Documents/go-test
    cd ~/Documents/go-test
    ```
2.  **Open in VS Code:** Type:
    ```bash
    code .
    ```
3.  **Initialize your module:** In the VS Code terminal (`Ctrl + ` ` `), type:
    ```bash
    go mod init test-app
    ```
4.  **Create a file:** Create `main.go` and paste:
    ```go
    package main
    import "fmt"

    func main() {
        fmt.Println("Hello, Mac Gopher! Setup complete.")
    }
    ```
5.  **Run it:**
    ```bash
    go run .
    ```


