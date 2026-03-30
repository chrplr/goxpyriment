# Mental Logic Card Game

A Go implementation of a behavioral experiment designed to test mental logic and inference.

## How to Run

By default, the experiment starts in **1280x1024 FULLSCREEN** mode.

### Standard Run (Fullscreen)
```bash
go run examples/card_game/main.go
```

### Development Mode (Windowed)
To run in a 1280x1024 window:
```bash
go run examples/card_game/main.go -w
```

### Command Line Options
- `-w`: Windowed mode (1024×768 window instead of fullscreen).
- `-d N`: Display ID — open on monitor N (-1 = primary).
- `--scaling <factor>`: Scaling factor for stimuli and layout (default: 1.0).
- `-F`: Force Fullscreen (default behavior).

## Instructions to the Subject

Three cards are going to be used throughout the experiment:

*   **Queen of Hearts** (red back)
*   **Black Spade Ace** (blue back)
*   **Red Diamond Ace** (blue back)

Note that only the queen card has a red back while the two ace cards have blue backs.

The experiment is a series of trials where the three cards are presented in a row. First their backs are presented, then one of the three cards is unmasked a second later.

Your task is to identify the **middle** card as quickly as possible, when possible. You will indicate your response by pressing a key:

*   **Queen** -> Press 'Q'
*   **Black Spade Ace** -> Press 'S'
*   **Red Diamond Ace** -> Press 'D'
*   **Don't know** -> Press 'N'

Now, rest your fingers on the keys 'Q', 'S', 'D' and 'N'.

When you are ready, press the **SPACE BAR** to start.

## Controls
- **ESC:** Interrupt and exit the experiment gracefully.
- **Q, S, D, N:** Response keys during trials.
- **SPACE:** Advance from the instruction screen.

## Data Collection
Results are saved as `.csv` files in the `data/` directory, logging trial conditions, response accuracy, and reaction times.
