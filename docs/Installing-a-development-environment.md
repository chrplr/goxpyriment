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
   - **Editor:** keep the default (you can change it later).
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

### 3. Editor

Any text editor works. If you are new to programming, start with one of the
simpler options below and switch later if you outgrow it.

**Notepad++ (simplest)** — tiny and instant, with Go syntax highlighting. Ideal if
you just want to edit a file (no autocomplete or error checking).

1. **Download:** [notepad-plus-plus.org](https://notepad-plus-plus.org/).
2. Run the installer and accept the defaults.

**Zed (lightweight, with Go support)** — a fast, uncluttered editor that still
gives you autocomplete and inline error checking.

1. **Download:** [zed.dev](https://zed.dev/).
2. Run the installer. The first time you open a `.go` file, Zed offers to install
   the Go tools — accept.

**Visual Studio Code (most popular, more features)** — powerful but can feel
overwhelming at first.

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

Install the latest version directly from the official site.

1. **Download** the Linux tarball from [go.dev/dl](https://go.dev/dl/):
   ```bash
   wget https://go.dev/dl/go1.26.4.linux-amd64.tar.gz
   ```
   Note: Replace the filename with the latest version shown on the downloads page (and the version for your computer arch (amd vs. arm)).

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

Do this only if you do not already have a favorite code editor. 

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

Once Go is installed, let's check that everything works by writing and running a
tiny program. If you have never used a command line before, don't worry — every
step is spelled out below.

### Step 0 — Open a terminal

Almost everything in Go is done from a **terminal** (also called a *command
line*, *command prompt*, or *shell*). This is a window where you type commands
and press **Enter** to run them, instead of clicking buttons. Open it like this:

- **Windows:** the easiest option is the **Git Bash** terminal that was installed
  together with Git. Click the **Start** menu, type `Git Bash`, and press
  **Enter**. (You can also use the built-in **Command Prompt**: press
  `Win + R`, type `cmd`, and press **Enter** — but the commands below assume Git
  Bash.)
- **macOS:** press `Cmd + Space`, type `Terminal`, and press **Enter**.
- **Linux (Ubuntu):** press `Ctrl + Alt + T`, or search for **Terminal** in the
  applications menu.

A new window opens with a blinking cursor, ready for you to type. From here on,
each time you see a command in a grey box, **type it into this terminal and press
Enter**.

> **Tip:** when a command box shows several lines, run them one at a time.

### Step 1 — Create a project folder

A Go program lives in its own folder. The commands below create a folder called
`hello`, move into it, and tell Go that this folder is a *module* (a self-
contained project):

```bash
mkdir hello
cd hello
go mod init hello
```

- `mkdir hello` — **m**a**k**e a new **dir**ectory (folder) named `hello`.
- `cd hello` — **c**hange **d**irectory: step inside the folder you just made.
  Every command you type from now on runs *inside* this folder.
- `go mod init hello` — set up the folder as a Go project.

> **Where did the folder go?** It is created inside whatever folder the terminal
> is currently "in" — usually your home folder (e.g. `C:\Users\YourName` on
> Windows, or `/home/yourname` on Linux). That is fine for now.

### Step 2 — Create the program file

Now create a text file named `main.go` inside the `hello` folder, using your
editor (VS Code, Zed, Notepad++, …). Open the editor, create a **new file**,
paste the lines below, and save it as `main.go` **inside the `hello` folder**:

```go
package main

import "fmt"

func main() {
    fmt.Println("Hello! Your environment is ready.")
}
```

> **Make sure the file is named exactly `main.go`** (not `main.go.txt`). In
> Notepad++ or VS Code, choose *Save As* and type the full name `main.go`.

### Step 3 — Run the program

Back in the terminal (still inside the `hello` folder), type:

```bash
go run .
```

The `.` means "run the program in the current folder". You should see this line
printed in the terminal:

```
Hello! Your environment is ready.
```

🎉 Congratulations — your Go environment is fully working, and you have just
written and run your first program.

---

## (Optional) Installing an AI Coding Agent: Claude Code

[Claude Code](https://claude.com/claude-code) is Anthropic's AI coding assistant
that runs **inside your terminal**. You describe what you want in plain English,
and it reads your files, writes code, runs commands, and explains what it is
doing. It is entirely optional, but many people find it a helpful companion when
learning Go or working with this project.

> **You will need an account.** Claude Code requires either a Claude
> subscription (Pro or Max) or an Anthropic API account with billing set up. The
> first time you run it, it walks you through signing in.

### Install

Open a **terminal** (see [Step 0](#step-0--open-a-terminal) above if you are not
sure how) and run the command for your platform.

- **macOS / Linux:**
  ```bash
  curl -fsSL https://claude.ai/install.sh | bash
  ```

- **Windows** — open **PowerShell** (click **Start**, type `PowerShell`, press
  **Enter**) and run:
  ```powershell
  irm https://claude.ai/install.ps1 | iex
  ```

Alternatively, on **any platform** that has [Node.js](https://nodejs.org/) 18 or
newer installed, you can install it with npm:

```bash
npm install -g @anthropic-ai/claude-code
```

> You may need to **close and reopen your terminal** after installing so that the
> `claude` command becomes available.

### First run

1. In your terminal, move into a project folder (for example the `hello` folder
   you created above, or your copy of this repository):
   ```bash
   cd hello
   ```
2. Start Claude Code:
   ```bash
   claude
   ```
3. The first time, it asks you to log in — follow the on-screen instructions to
   sign in with your Claude or Anthropic account.
4. Once you see the prompt, just type a request in plain English and press
   **Enter**, for example:
   ```
   Explain what the main.go file does, line by line.
   ```

To leave Claude Code, type `/exit` (or press `Ctrl + C` twice).

> **Verify the install** at any time with:
> ```bash
> claude --version
> ```
