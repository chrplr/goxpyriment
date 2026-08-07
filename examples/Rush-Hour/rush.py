import sys
import pygame

# Initialize Pygame
pygame.init()

# Constants
GRID_SIZE = 6
TILE_SIZE = 90
BOARD_MARGIN = 20
WINDOW_WIDTH = GRID_SIZE * TILE_SIZE + (BOARD_MARGIN * 2)
WINDOW_HEIGHT = GRID_SIZE * TILE_SIZE + (BOARD_MARGIN * 2) + 60  # Extra space for status text

# Colors (RGB)
BG_COLOR = (240, 240, 240)
GRID_COLOR = (180, 180, 180)
EXIT_COLOR = (220, 50, 50)
TEXT_COLOR = (30, 30, 30)

CAR_COLORS = [
    (220, 50, 50),   # Red car (Target)
    (50, 120, 220),  # Blue
    (50, 180, 80),   # Green
    (220, 160, 40),  # Orange
    (140, 60, 200),  # Purple
    (200, 200, 50),  # Yellow
    (50, 180, 200),  # Cyan
]

class Car:
    def __init__(self, car_id, row, col, length, orientation, is_target=False):
        self.id = car_id
        self.row = row
        self.col = col
        self.length = length
        self.orientation = orientation  # 'H' for horizontal, 'V' for vertical
        self.is_target = is_target
        self.color = CAR_COLORS[0] if is_target else CAR_COLORS[(car_id % (len(CAR_COLORS) - 1)) + 1]

class RushHourGame:
    def __init__(self):
        self.screen = pygame.display.set_mode((WINDOW_WIDTH, WINDOW_HEIGHT))
        pygame.display.set_caption("Rush Hour - Sliding Block Puzzle")
        self.clock = pygame.time.Clock()
        self.font = pygame.font.SysFont(None, 36)
        
        self.selected_car = None
        self.drag_offset_x = 0
        self.drag_offset_y = 0
        self.won = False
        
        # Load default puzzle (Red car is at row 2, col 1)
        self.cars = [
            Car(0, row=2, col=1, length=2, orientation='H', is_target=True),  # Target Red Car
            Car(1, row=0, col=0, length=2, orientation='V'),
            Car(2, row=0, col=1, length=3, orientation='H'),
            Car(3, row=1, col=4, length=2, orientation='V'),
            Car(4, row=2, col=3, length=3, orientation='V'),
            Car(5, row=4, col=0, length=2, orientation='H'),
            Car(6, row=5, col=2, length=3, orientation='H'),
        ]

    def get_grid(self, exclude_car=None):
        grid = [[None for _ in range(GRID_SIZE)] for _ in range(GRID_SIZE)]
        for car in self.cars:
            if car == exclude_car:
                continue
            for i in range(car.length):
                r = car.row + (i if car.orientation == 'V' else 0)
                c = car.col + (i if car.orientation == 'H' else 0)
                if 0 <= r < GRID_SIZE and 0 <= c < GRID_SIZE:
                    grid[r][c] = car.id
        return grid

    def try_move_car(self, car, new_row, new_col):
        if car.orientation == 'H' and new_row != car.row:
            return
        if car.orientation == 'V' and new_col != car.col:
            return

        grid = self.get_grid(exclude_car=car)
        
        # Step-by-step collision check along movement vector
        step_r = 1 if new_row > car.row else -1 if new_row < car.row else 0
        step_c = 1 if new_col > car.col else -1 if new_col < car.col else 0

        curr_r, curr_c = car.row, car.col
        while (curr_r, curr_c) != (new_row, new_col):
            next_r = curr_r + step_r
            next_c = curr_c + step_c
            
            # Boundary checks
            if not (0 <= next_r <= GRID_SIZE - (car.length if car.orientation == 'V' else 1)):
                break
            if not (0 <= next_c <= GRID_SIZE - (car.length if car.orientation == 'H' else 1)):
                break
                
            # Collision checks for the entire vehicle span
            blocked = False
            for i in range(car.length):
                check_r = next_r + (i if car.orientation == 'V' else 0)
                check_c = next_c + (i if car.orientation == 'H' else 0)
                if grid[check_r][check_c] is not None:
                    blocked = True
                    break
            
            if blocked:
                break
                
            curr_r, curr_c = next_r, next_c

        car.row, car.col = curr_r, curr_c

        # Win condition: Red car reaches the right wall (row 2, col 4)
        if car.is_target and car.col == 4:
            self.won = True

    def run(self):
        while True:
            for event in pygame.event.get():
                if event.type == pygame.QUIT:
                    pygame.quit()
                    sys.exit()

                if event.type == pygame.MOUSEBUTTONDOWN and not self.won:
                    mx, my = event.pos
                    # Identify clicked car
                    for car in self.cars:
                        x = BOARD_MARGIN + car.col * TILE_SIZE
                        y = BOARD_MARGIN + car.row * TILE_SIZE
                        w = (car.length * TILE_SIZE if car.orientation == 'H' else TILE_SIZE)
                        h = (TILE_SIZE if car.orientation == 'H' else car.length * TILE_SIZE)
                        if pygame.Rect(x, y, w, h).collidepoint(mx, my):
                            self.selected_car = car
                            break

                elif event.type == pygame.MOUSEBUTTONUP:
                    self.selected_car = None

                elif event.type == pygame.MOUSEMOTION and self.selected_car and not self.won:
                    mx, my = event.pos
                    grid_col = max(0, min(GRID_SIZE - 1, (mx - BOARD_MARGIN) // TILE_SIZE))
                    grid_row = max(0, min(GRID_SIZE - 1, (my - BOARD_MARGIN) // TILE_SIZE))
                    
                    self.try_move_car(self.selected_car, grid_row, grid_col)

            self.draw()
            self.clock.tick(60)

    def draw(self):
        self.screen.fill(BG_COLOR)

        # Draw Grid Board
        for r in range(GRID_SIZE):
            for c in range(GRID_SIZE):
                rect = pygame.Rect(
                    BOARD_MARGIN + c * TILE_SIZE,
                    BOARD_MARGIN + r * TILE_SIZE,
                    TILE_SIZE,
                    TILE_SIZE
                )
                pygame.draw.rect(self.screen, GRID_COLOR, rect, 1)

        # Highlight Exit (Row 2, Column 5 right edge)
        exit_rect = pygame.Rect(
            BOARD_MARGIN + 6 * TILE_SIZE - 6,
            BOARD_MARGIN + 2 * TILE_SIZE,
            8,
            TILE_SIZE
        )
        pygame.draw.rect(self.screen, EXIT_COLOR, exit_rect)

        # Draw Cars
        for car in self.cars:
            x = BOARD_MARGIN + car.col * TILE_SIZE + 4
            y = BOARD_MARGIN + car.row * TILE_SIZE + 4
            w = (car.length * TILE_SIZE - 8 if car.orientation == 'H' else TILE_SIZE - 8)
            h = (TILE_SIZE - 8 if car.orientation == 'H' else car.length * TILE_SIZE - 8)
            
            rect = pygame.Rect(x, y, w, h)
            pygame.draw.rect(self.screen, car.color, rect, border_radius=8)
            pygame.draw.rect(self.screen, (0, 0, 0), rect, width=2, border_radius=8)

        # Draw Text / Status
        status_text = "PUZZLE SOLVED!" if self.won else "Drag vehicles to clear the path for RED."
        color = EXIT_COLOR if self.won else TEXT_COLOR
        text_surface = self.font.render(status_text, True, color)
        self.screen.blit(text_surface, (BOARD_MARGIN, WINDOW_HEIGHT - 45))

        pygame.display.flip()

if __name__ == "__main__":
    game = RushHourGame()
    game.run()