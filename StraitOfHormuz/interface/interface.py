import json
import pygame

MARGIN = 40
SCALE = 4

SECTORS_PATH = "../data/interface/interface_sectors.json"
DRONES_PATH = "../data/interface/interface_drones.json"
SENSORS_PATH = "../data/interface/interface_sensors.json"

DRONE_IMAGE_PATH = "../data/images/drone.webp"


def load_list(path):
    try:
        with open(path, "r", encoding="utf-8") as file:
            data = json.load(file)

        if data is None:
            return []

        return data

    except:
        return []


def load_world():
    return {
        "sectors": load_list(SECTORS_PATH),
        "drones": load_list(DRONES_PATH),
        "sensors": load_list(SENSORS_PATH),
    }


def get_world_limits(sectors):
    if not sectors:
        return 0, 300, 0, 100

    min_x = min(min(sector["left"], sector["right"]) for sector in sectors)
    max_x = max(max(sector["left"], sector["right"]) for sector in sectors)

    min_y = min(min(sector["top"], sector["bottom"]) for sector in sectors)
    max_y = max(max(sector["top"], sector["bottom"]) for sector in sectors)

    return min_x, max_x, min_y, max_y


def get_screen_size(min_x, max_x, min_y, max_y):
    width = int((max_x - min_x) * SCALE + 2 * MARGIN)
    height = int((max_y - min_y) * SCALE + 2 * MARGIN)

    return width, height


def world_to_screen(x, y, min_x, max_y):
    screen_x = MARGIN + (x - min_x) * SCALE
    screen_y = MARGIN + (max_y - y) * SCALE

    return int(screen_x), int(screen_y)


def draw_sectors(screen, font, sectors, min_x, max_y):
    for sector in sectors:
        left, top = world_to_screen(sector["left"], sector["top"], min_x, max_y)
        right, bottom = world_to_screen(sector["right"], sector["bottom"], min_x, max_y)

        rect_left = min(left, right)
        rect_top = min(top, bottom)
        rect_width = abs(right - left)
        rect_height = abs(bottom - top)

        rect = pygame.Rect(rect_left, rect_top, rect_width, rect_height)

        pygame.draw.rect(screen, (60, 60, 60), rect, 2)

        text = font.render(f"Setor {sector.get('ID', sector.get('id', '?'))}", True, (200, 200, 200))
        screen.blit(text, (rect_left + 10, rect_top + 10))


def draw_sensors(screen, font, sensors, min_x, max_y):
    for sensor in sensors:
        x, y = world_to_screen(sensor["x"], sensor["y"], min_x, max_y)

        color = (0, 180, 255)
        radius = 6

        pygame.draw.circle(screen, color, (x, y), radius)

        text = font.render("S", True, (255, 255, 255))
        screen.blit(text, (x + 8, y - 8))


def draw_drones(screen, font, drones, drone_image, min_x, max_y):
    for drone in drones:
        x, y = world_to_screen(drone["x"], drone["y"], min_x, max_y)

        if drone_image is not None:
            image_rect = drone_image.get_rect(center=(x, y))
            screen.blit(drone_image, image_rect)
        else:
            pygame.draw.circle(screen, (100, 255, 100), (x, y), 12)

        text = font.render(f"D{drone.get('id', '?')}", True, (255, 255, 255))
        screen.blit(text, (x + 14, y - 8))


def draw_grid(screen, font, min_x, max_x, min_y, max_y, width, height):
    start_x = int(min_x)
    end_x = int(max_x)

    for x in range(start_x, end_x + 1, 50):
        sx, _ = world_to_screen(x, min_y, min_x, max_y)

        pygame.draw.line(screen, (40, 40, 40), (sx, MARGIN), (sx, height - MARGIN), 1)

        text = font.render(str(x), True, (150, 150, 150))
        screen.blit(text, (sx - 10, height - MARGIN + 5))

    start_y = int(min_y)
    end_y = int(max_y)

    for y in range(start_y, end_y + 1, 20):
        _, sy = world_to_screen(min_x, y, min_x, max_y)

        pygame.draw.line(screen, (40, 40, 40), (MARGIN, sy), (width - MARGIN, sy), 1)

        text = font.render(str(y), True, (150, 150, 150))
        screen.blit(text, (5, sy - 8))


def main():
    pygame.init()

    world = load_world()
    sectors = world.get("sectors") or []

    min_x, max_x, min_y, max_y = get_world_limits(sectors)
    width, height = get_screen_size(min_x, max_x, min_y, max_y)

    screen = pygame.display.set_mode((width, height), pygame.RESIZABLE)
    pygame.display.set_caption("Monitoramento - Strait of Hormuz")

    font = pygame.font.SysFont("Arial", 16)
    clock = pygame.time.Clock()

    try:
        drone_image = pygame.image.load(DRONE_IMAGE_PATH).convert_alpha()
        drone_image = pygame.transform.scale(drone_image, (32, 32))
    except:
        drone_image = None

    running = True

    while running:
        for event in pygame.event.get():
            if event.type == pygame.QUIT:
                running = False

        world = load_world()

        sectors = world.get("sectors") or []
        sensors = world.get("sensors") or []
        drones = world.get("drones") or []

        new_min_x, new_max_x, new_min_y, new_max_y = get_world_limits(sectors)
        new_width, new_height = get_screen_size(new_min_x, new_max_x, new_min_y, new_max_y)

        if new_width != width or new_height != height:
            min_x = new_min_x
            max_x = new_max_x
            min_y = new_min_y
            max_y = new_max_y

            width = new_width
            height = new_height

            screen = pygame.display.set_mode((width, height), pygame.RESIZABLE)

        screen.fill((20, 20, 20))

        draw_grid(screen, font, min_x, max_x, min_y, max_y, width, height)
        draw_sectors(screen, font, sectors, min_x, max_y)
        draw_sensors(screen, font, sensors, min_x, max_y)
        draw_drones(screen, font, drones, drone_image, min_x, max_y)

        pygame.display.flip()
        clock.tick(30)

    pygame.quit()


if __name__ == "__main__":
    main()