It looks like you’ve built a robust foundation for **goxpyriment**. Your current list covers the "greatest hits" of cognitive psychology, but to make the framework truly comprehensive, you could expand into areas like executive function, signal detection, and more complex social/emotional processing.

Here are a few classic paradigms that would be excellent additions to your examples:

## 1. Executive Function & Inhibitory Control
Since you have the Stroop and Simon tasks, adding these would round out the "conflict" and "switching" categories:

* **Flanker Task (Eriksen & Eriksen, 1974):** Participants respond to a central target (e.g., `<` or `>`) while ignoring surrounding "flankers." It's a classic measure of selective attention and spatial interference.
* **Go/No-Go Task:** Requires the participant to respond to a frequent stimulus but withhold the response for a rare "No-Go" stimulus. This is the gold standard for measuring response inhibition.
* **Wisconsin Card Sorting Task (WCST) / Rule Switching:** A task where the "correct" sorting rule (color, shape, or number) changes unexpectedly. This would demonstrate how your framework handles complex state-based logic.

## 2. Signal Detection & Psychophysics
You have the Magnitude Estimation and Sensory Threshold tasks, but these would add more depth to your psychophysics suite:

* **Change Blindness (Flicker Paradigm):** Two versions of an image are shown in rapid alternation with a brief blank mask in between. This is great for demonstrating how your framework handles high-res image stimuli and timing. 
* **Dot-Probe Task:** Used to measure attentional bias (often toward emotional stimuli). Two stimuli (e.g., a neutral face and a fearful face) appear, followed by a dot in one of those locations.
* **Global/Local Task (Navon, 1977):** Large letters made out of smaller letters (e.g., a large "H" made of small "S"s). This tests "forest vs. trees" visual processing.

## 3. Working Memory & Mental Representation
Your list has the Sternberg and Memory Span tasks; adding these would cover more modern working memory metrics:

* **N-Back Task:** The participant must decide if the current stimulus matches the one shown *n* steps ago. It’s the "stress test" for working memory and would be a great way to show how you handle variable back-end arrays.
* **Corsi Block-Tapping Task:** The visual-spatial version of a digit span task. This would demonstrate how your framework handles 2D spatial layouts and mouse/touch input sequences.
* **Mental Chronometry (Hick’s Law):** A task measuring RT as a function of the number of possible choices. It’s a very simple but foundational experiment for any new library.

## 4. Learning & Memory
* **Deese-Roediger-McDermott (DRM) Paradigm:** Participants study a list of related words (e.g., *bed, rest, awake*) and often "falsely" remember seeing a related but unpresented lure (*sleep*). Excellent for showing text-based list randomization.
* **Serial Position Effect:** A simple free-recall task to demonstrate primacy and recency effects.

---

### Implementation Suggestion:
A great way to show off the power of a Go-based framework (which I assume is performant and handles concurrency well) would be a **Dual-Task Paradigm**. For example, having the participant perform a **Simple Reaction Time** task while simultaneously keeping a **Memory Span** list in mind. This demonstrates how your framework handles overlapping stimulus timers and independent input streams.

Since **goxpyriment** is built in Go, a dual-task paradigm is a brilliant way to showcase the language's strengths in handling concurrent events and precise timing without dropping frames.

A classic implementation is the **Psychological Refractory Period (PRP)** effect. In this setup, the participant must perform two independent tasks that overlap in time.

### The PRP Dual-Task Design
The goal is to show that as the **Stimulus Onset Asynchrony (SOA)**—the delay between Task 1 and Task 2—decreases, the reaction time for the second task ($RT_2$) increases linearly.



#### 1. The Stimuli & Tasks
* **Task 1 (Auditory):** A high-pitched or low-pitched tone.
    * *Response:* Press 'S' for Low, 'D' for High.
* **Task 2 (Visual):** A letter ('O' or 'X') appears on the screen.
    * *Response:* Press 'K' for O, 'L' for X.

#### 2. Experimental Logic (Pseudocode)
In your framework, you would likely structure the trial loop to handle the "offset" start of the second stimulus.

```go
// Example logic for a single PRP trial
func RunTrial(soa time.Duration) {
    // 1. Present Auditory Stimulus (Task 1)
    playTone(toneType)
    t1Start := time.Now()

    // 2. Wait for the SOA (Stimulus Onset Asynchrony)
    // This is where Go's concurrency shines; we can 
    // listen for T1 response while waiting to show T2.
    time.Sleep(soa)

    // 3. Present Visual Stimulus (Task 2)
    showLetter(letterType)
    t2Start := time.Now()

    // 4. Collect both responses asynchronously
    // You'll need to capture the delta from t1Start and t2Start
}
```

---

### Why this is a "Power User" Example:
* **Bottleneck Theory:** It demonstrates the "central bottleneck" in human cognition. Even if the hands and eyes are free, the brain can only "program" one response at a time.
* **Concurrency Stress Test:** It tests if your framework can accurately log a keypress for Task 1 while simultaneously triggering the visual draw for Task 2.
* **Data Analysis:** It provides a beautiful linear graph: as $SOA$ $\to$ 0, $RT_2$ $\to$ $\infty$.

### Suggested Stimulus Table for your CSV/JSON:
| Trial | Task 1 (Tone) | Task 2 (Visual) | SOA (ms) | Expected Result |
| :--- | :--- | :--- | :--- | :--- |
| 1 | 440 Hz | "X" | 50 | High $RT_2$ (Interference) |
| 2 | 880 Hz | "O" | 400 | Medium $RT_2$ |
| 3 | 440 Hz | "X" | 1000 | Low $RT_2$ (No Interference) |

Would you like me to help you refine the **Go code** specifically for handling these overlapping input listeners, or should we look at a different paradigm like the **N-Back**?
