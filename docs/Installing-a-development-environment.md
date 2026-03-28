# Installing a Go Development Environment

This guide walks you through setting up **Git**, **Go**, and a code editor on **macOS**, **Windows**, and **Linux (Ubuntu)**.

- [macOS](#-macos)
- [Windows](#-windows)
- [Linux (Ubuntu)](#-linux-ubuntu)

---

## 🍎 macOS

If [Homebrew](https://brew.sh/) is installed, you can install Git and Go with two commands:

```bash
brew install git
brew install go
```

Otherwise, follow the steps below.

### 1. Install Apple Command Line Tools

Before installing anything else, macOS needs its basic developer foundation.

1. Open your **Terminal** (press `Cmd + Space` and type "Terminal").
2. Run:
   ```bash
   xcode-select --install
   ```
3. A popup will appear — click **Install** and agree to the terms.

### 2. Install Git

macOS ships with an outdated Git; install the official version.

1. **Download:** Go to [git-scm.com/download/mac](https://git-scm.com/download/mac).
2. Use the **binary installer** package.
3. **Verify:**
   ```bash
   git --version
   ```

### 3. Install Go

1. **Download:** Visit [go.dev/dl](https://go.dev/dl/) and select the **macOS** installer (`.pkg`).
   - Apple Silicon (M1/M2/M3/…): choose **ARM64**.
   - Intel Mac: choose **x86-64**.
2. Open the package and follow the wizard.
3. **Verify** (open a **new** Terminal window):
   ```bash
   go version
   ```

### 4. Editor: Visual Studio Code (VS Code)

If you do not already have a favourite editor (Vim or Emacs work great too), install VS Code.

1. **Download:** [code.visualstudio.com](https://code.visualstudio.com/).
2. Unzip the download and drag **Visual Studio Code** into your Applications folder.
3. **Add the `code` command to PATH:**
   - Open VS Code, press `Cmd + Shift + P`.
   - Type `shell command` and select **"Shell Command: Install 'code' command in PATH"**.
4. **Install the Go extension:**
   - Click the **Extensions** icon (four squares) in the left sidebar.
   - Search for **"Go"** and install the one by the Go Team at Google.

---

## 🪟 Windows

### 1. Install Git

1. **Download:** [git-scm.com/download/win](https://git-scm.com/download/win).
2. Run the `.exe` installer. On most screens you can click **Next**, but note:
   - **Editor:** choose "Visual Studio Code as Git's default editor" (if already installed).
   - **PATH:** select "Git from the command line and also from 3rd-party software".
   - **Line endings:** choose "Checkout Windows-style, commit Unix-style line endings".
3. **Verify** — open **Command Prompt** (`Win + R`, type `cmd`) and run:
   ```bash
   git --version
   ```

### 2. Install Go

1. **Download:** [go.dev/dl](https://go.dev/dl/) — choose the **Windows** installer (`.msi`).
2. Run the installer; it defaults to `C:\Program Files\Go`.
3. Close any open Command Prompts, then open a new one.
4. **Verify:**
   ```bash
   go version
   ```
   You should see something like `go version go1.26.x windows/amd64`.

### 3. Editor: Visual Studio Code (VS Code)

1. **Download:** [code.visualstudio.com](https://code.visualstudio.com/).
2. Run the installer and make sure **"Add to PATH"** is checked.
3. **Set up Go support:**
   - Open VS Code and click the **Extensions** icon (four squares).
   - Search for **"Go"** (by the Go Team at Google) and click **Install**.
   - Press `Ctrl + Shift + P`, type `Go: Install/Update Tools`, select all boxes, and click **OK**.

---

## 🐧 Linux (Ubuntu)

### 1. Install Git

Git is available in Ubuntu's package repository:

```bash
sudo apt update
sudo apt install git
```

**Verify:**
```bash
git --version
```

### 2. Install Go

Ubuntu's packaged Go is often outdated. Install the latest version directly from the official site.

1. **Download** the Linux tarball from [go.dev/dl](https://go.dev/dl/) (choose the `linux-amd64` build for most machines):
   ```bash
   wget https://go.dev/dl/go1.24.3.linux-amd64.tar.gz
   ```
   Replace the filename with the latest version shown on the downloads page.

2. **Extract** to `/usr/local` (this is the standard location):
   ```bash
   sudo rm -rf /usr/local/go
   sudo tar -C /usr/local -xzf go1.24.3.linux-amd64.tar.gz
   ```

3. **Add Go to your PATH.** Append the following lines to `~/.bashrc` (or `~/.zshrc` if you use Zsh):
   ```bash
   export PATH=$PATH:/usr/local/go/bin
   ```
   Then reload your shell:
   ```bash
   source ~/.bashrc
   ```

4. **Verify:**
   ```bash
   go version
   ```

### 3. Editor: Visual Studio Code (VS Code)

1. **Install** via Snap (simplest method):
   ```bash
   sudo snap install --classic code
   ```
   Alternatively, download the `.deb` package from [code.visualstudio.com](https://code.visualstudio.com/) and install with `sudo apt install ./code_*.deb`.

2. **Install the Go extension:**
   - Click the **Extensions** icon (four squares) in the left sidebar.
   - Search for **"Go"** and install the one by the Go Team at Google.
   - Press `Ctrl + Shift + P`, type `Go: Install/Update Tools`, select all, and click **OK**.

---

## Your First Program (Hello World)

Once Go is installed on any platform, verify everything works:

1. Create a project folder and initialise a module:
   ```bash
   mkdir hello && cd hello
   go mod init hello
   ```

2. Create `main.go`:
   ```go
   package main
   import "fmt"

   func main() {
       fmt.Println("Hello, Gopher! Your environment is ready.")
   }
   ```

3. Run it:
   ```bash
   go run .
   ```

You should see `Hello, Gopher! Your environment is ready.` printed in the terminal.
