import json
import pygame

WIDTH = 900
HEIGHT = 400
MARGIN = 40

WORLD_PATH = "../data/world.json"

WORLD_MIN_X = 0
WORLD_MAX_X = 300
WORLD_MIN_Y = 0
WORLD_MAX_Y = 100


def load_world():
    try:
        with open(WORLD_PATH, "r", encoding="utf-8") as file:
            return json.load(file)
    except:
        return {
            "drones": [],
            "sensors": [],
            "sectors": []
        }


def world_to_screen(x, y):
    screen_x = MARGIN + (x - WORLD_MIN_X) / (WORLD_MAX_X - WORLD_MIN_X) * (WIDTH - 2 * MARGIN)
    screen_y = HEIGHT - MARGIN - (y - WORLD_MIN_Y) / (WORLD_MAX_Y - WORLD_MIN_Y) * (HEIGHT - 2 * MARGIN)

    return int(screen_x), int(screen_y)


def draw_sectors(screen, font, sectors):
    for sector in sectors:
        left, top = world_to_screen(sector["left"], sector["top"])
        right, bottom = world_to_screen(sector["right"], sector["bottom"])

        rect = pygame.Rect(left, top, right - left, bottom - top)

        pygame.draw.rect(screen, (60, 60, 60), rect, 2)

        text = font.render(f"Setor {sector.get('ID', sector.get('id', '?'))}", True, (200, 200, 200))
        screen.blit(text, (left + 10, top + 10))


def draw_sensors(screen, font, sensors):
    for sensor in sensors:
        x, y = world_to_screen(sensor["x"], sensor["y"])

        if sensor.get("is_critical"):
            color = (255, 0, 0)
            radius = 9
        elif sensor.get("is_active"):
            color = (255, 200, 0)
            radius = 7
        else:
            color = (0, 180, 255)
            radius = 5

        pygame.draw.circle(screen, color, (x, y), radius)

        text = font.render("S", True, (255, 255, 255))
        screen.blit(text, (x + 8, y - 8))


def draw_drones(screen, font, drones):
    for drone in drones:
        x, y = world_to_screen(drone["x"], drone["y"])

        if drone.get("is_busy"):
            color = (255, 100, 100)
        else:
            color = (100, 255, 100)

        pygame.draw.circle(screen, color, (x, y), 12)

        text = font.render(f"D{drone.get('id', '?')}", True, (0, 0, 0))
        screen.blit(text, (x - 10, y - 8))


def draw_grid(screen, font):
    for x in range(WORLD_MIN_X, WORLD_MAX_X + 1, 50):
        sx, _ = world_to_screen(x, 0)
        pygame.draw.line(screen, (40, 40, 40), (sx, MARGIN), (sx, HEIGHT - MARGIN), 1)

        text = font.render(str(x), True, (150, 150, 150))
        screen.blit(text, (sx - 10, HEIGHT - MARGIN + 5))

    for y in range(WORLD_MIN_Y, WORLD_MAX_Y + 1, 20):
        _, sy = world_to_screen(0, y)
        pygame.draw.line(screen, (40, 40, 40), (MARGIN, sy), (WIDTH - MARGIN, sy), 1)

        text = font.render(str(y), True, (150, 150, 150))
        screen.blit(text, (5, sy - 8))


def main():
    pygame.init()

    screen = pygame.display.set_mode((WIDTH, HEIGHT))
    pygame.display.set_caption("Monitoramento - Strait of Hormuz")

    font = pygame.font.SysFont("Arial", 16)
    clock = pygame.time.Clock()

    running = True

    while running:
        for event in pygame.event.get():
            if event.type == pygame.QUIT:
                running = False

        world = load_world()

        screen.fill((20, 20, 20))

        draw_grid(screen, font)
        draw_sectors(screen, font, world.get("sectors", []))
        draw_sensors(screen, font, world.get("sensors", []))
        draw_drones(screen, font, world.get("drones", []))

        pygame.display.flip()
        clock.tick(30)

    pygame.quit()


if __name__ == "__main__":
    main()