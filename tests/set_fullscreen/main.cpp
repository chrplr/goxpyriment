#include <SDL3/SDL.h>
#include <iostream>
#include <vector>

int main(int argc, char* argv[]) {
    // 1. Initialize SDL Video
    if (!SDL_Init(SDL_INIT_VIDEO)) {
        SDL_Log("SDL_Init failed: %s", SDL_GetError());
        return -1;
    }

    // 2. Create a standard window
    // Initially, it's just a normal windowed window.
    SDL_Window* window = SDL_CreateWindow("SDL3 Exclusive Fullscreen", 1280, 720, SDL_WINDOW_RESIZABLE);
    if (!window) {
        SDL_Log("Window creation failed: %s", SDL_GetError());
        SDL_Quit();
        return -1;
    }

    // 3. Get the Display ID for the window
    SDL_DisplayID displayID = SDL_GetDisplayForWindow(window);
    if (displayID == 0) {
        SDL_Log("Could not get DisplayID: %s", SDL_GetError());
    }

    // 4. Determine the possible resolutions (Display Modes)
    int modeCount = 0;
    // SDL_GetFullscreenDisplayModes returns a NULL-terminated array of pointers.
    // You are responsible for calling SDL_free() on the returned pointer.
    SDL_DisplayMode** modes = SDL_GetFullscreenDisplayModes(displayID, &modeCount);
    
    if (modes) {
        std::cout << "Available Hardware Modes:\n";
        for (int i = 0; i < modeCount; ++i) {
            std::cout << "[" << i << "] " << modes[i]->w << "x" << modes[i]->h 
                      << " @ " << modes[i]->refresh_rate << "Hz\n";
        }
        // Memory management: free the array (not the modes inside, which SDL manages)
        SDL_free(modes);
    }

    // 5. Get the CURRENT Desktop Mode
    // To bypass the compositor at native resolution, we fetch the desktop's current mode.
    const SDL_DisplayMode* desktopMode = SDL_GetDesktopDisplayMode(displayID);
    if (desktopMode) {
        std::cout << "\nSetting Exclusive Mode to Native: " 
                  << desktopMode->w << "x" << desktopMode->h << std::endl;

        // 6. Set the window's specific fullscreen mode
        // Passing a mode here triggers "Exclusive" behavior.
        // If you passed NULL here, it would use "Fullscreen Desktop" (borderless).
        SDL_SetWindowFullscreenMode(window, desktopMode);
    }

    // 7. Trigger the Fullscreen state
    // In SDL3, this is a simple boolean toggle.
    if (!SDL_SetWindowFullscreen(window, true)) {
        SDL_Log("Could not set fullscreen: %s", SDL_GetError());
    }

    // Main Loop
    bool running = true;
    SDL_Event event;
    while (running) {
        while (SDL_PollEvent(&event)) {
            if (event.type == SDL_EVENT_QUIT) {
                running = false;
            }
            if (event.type == SDL_EVENT_KEY_DOWN) {
                if (event.key.key == SDLK_ESCAPE) {
                    running = false;
                }
            }
        }
        
        // Render stuff here...
    }

    SDL_DestroyWindow(window);
    SDL_Quit();
    return 0;
}
