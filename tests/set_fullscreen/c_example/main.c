#include <SDL3/SDL.h>
#include <SDL3/SDL_main.h>

int main(int argc, char* argv[]) {
    if (SDL_Init(SDL_INIT_VIDEO) < 0) return -1;

    // 1. Create window with High-DPI and Fullscreen flags
    // Using 0,0 for width/height with FULLSCREEN often defaults to native desktop res
    SDL_Window* window = SDL_CreateWindow("Physical Res Demo", 0, 0, 
                                          SDL_WINDOW_HIGH_PIXEL_DENSITY | SDL_WINDOW_FULLSCREEN);
    
    SDL_Renderer* renderer = SDL_CreateRenderer(window, NULL);

    // 2. Query the actual physical pixel dimensions
    int PhysW, PhysH;
    SDL_GetWindowSizeInPixels(window, &PhysW, &PhysH);

    // 3. Get the scale factor (Density) to fix input coordinates
    // If your screen is 200% scaling, this returns 2.0f
    float pixelDensity = SDL_GetWindowPixelDensity(window);

    bool running = true;
    SDL_Event event;

    while (running) {
        while (SDL_PollEvent(&event)) {
            if (event.type == SDL_EVENT_QUIT) running = false;
            if (event.type == SDL_EVENT_KEY_DOWN && event.key.key == SDLK_ESCAPE) running = false;
        }

        // --- INPUT MAPPING ---
        float logicalMouseX, logicalMouseY;
        SDL_GetMouseState(&logicalMouseX, &logicalMouseY);

        // Map logical mouse coordinates to physical pixels
        float physMouseX = logicalMouseX * pixelDensity;
        float physMouseY = logicalMouseY * pixelDensity;

        // --- RENDERING ---
        SDL_SetRenderDrawColor(renderer, 20, 20, 20, 255); // Dark Gray
        SDL_RenderClear(renderer);

        // Draw a 1-pixel thick border around the physical edge
        SDL_SetRenderDrawColor(renderer, 255, 255, 255, 255);
        SDL_FRect screenRect = { 0, 0, (float)PhysW, (float)PhysH };
        SDL_RenderDebugText(renderer, 10, 10, "Bypassing Scaling");
        
        // Draw a small square at the physical mouse position
        SDL_FRect mouseRect = { physMouseX - 5, physMouseY - 5, 10, 10 };
        SDL_RenderFillRect(renderer, &mouseRect);

        SDL_RenderPresent(renderer);
    }

    SDL_DestroyRenderer(renderer);
    SDL_DestroyWindow(window);
    SDL_Quit();
    return 0;
}
