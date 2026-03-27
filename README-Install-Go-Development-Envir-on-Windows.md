This README is designed for a "from scratch" setup on Windows. 

---

# 🚀 Setting Up Your Go Development Environment (Windows)

This guide will help you install the three essential tools you need to start programming in **Go**: a Version Control System (**Git**), the **Go** language itself, and a modern **Code Editor**.

## 1. Install Git
Git tracks changes in your code and is essential for collaborating with others.

1.  **Download:** Go to [git-scm.com](https://git-scm.com/download/win).
2.  **Run the Installer:** Open the `.exe` file you just downloaded.
3.  **Key Settings:** You can click "Next" on most screens, but ensure these are selected:
    * **Editor:** Choose "Select Visual Studio Code as Git's default editor" (if you've installed it already) or leave it as Vim/Notepad.
    * **PATH Environment:** Ensure "Git from the command line and also from 3rd-party software" is selected.
    * **Line Endings:** Choose "Checkout Windows-style, commit Unix-style line endings."
4.  **Verify:** Open the **Command Prompt** (type `cmd` in your Start menu) and type:
    ```bash
    git --version
    ```

## 2. Install Go (Golang)
This is the engine that actually runs and compiles your code.

1.  **Download:** Visit [go.dev/dl](https://go.dev/dl/) and select the **Microsoft Windows** installer (`.msi`).
2.  **Install:** Run the installer and follow the prompts. It will default to `C:\Program Files\Go`.
3.  **Restart Terminal:** Close any open Command Prompts and open a new one to refresh your settings.
4.  **Verify:** Type the following in your Command Prompt:
    ```bash
    go version
    ```
    *You should see something like `go version go1.26.x windows/amd64`.*

## 3. Recommended Editor: Visual Studio Code (VS Code)
While there are many editors, **Visual Studio Code** is the gold standard for beginners and pros alike because of its "Go extension" which does half the work for you.

1.  **Download:** Get it at [code.visualstudio.com](https://code.visualstudio.com/).
2.  **Install:** Run the setup and make sure "Add to PATH" is checked.
3.  **Setup for Go:**
    * Open VS Code.
    * Click the **Extensions** icon on the left sidebar (it looks like four squares).
    * Search for **"Go"** (the one by the Go Team at Google) and click **Install**.
    * **Crucial Step:** Press `Ctrl + Shift + P`, type `Go: Install/Update Tools`, select all boxes, and click **OK**. This installs the helpers that provide "autocomplete" and error checking.

---

## 🏗️ Your First Program (The "Hello World" Test)
Let's make sure everything works.

1.  **Create a folder:** Create a new folder anywhere (e.g., `C:\Projects\hello`).
2.  **Open in VS Code:** Right-click the folder and select "Open with Code".
3.  **Initialize your module:** Open the terminal inside VS Code (`Ctrl + ` ` `) and type:
    ```bash
    go mod init hello
    ```
4.  **Create a file:** Create a new file named `main.go` and paste this:
    ```go
    package main
    import "fmt"

    func main() {
        fmt.Println("Hello, Gophers! Your environment is ready.")
    }
    ```
5.  **Run it:** In the terminal, type:
    ```bash
    go run .
    ```

> 

Gemini
March 27, 2026




