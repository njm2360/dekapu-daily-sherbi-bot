import os
import logging

from dotenv import load_dotenv

from bot import create_client
from settings import SettingsRepository

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
)

load_dotenv()

if __name__ == "__main__":
    os.makedirs("data", exist_ok=True)
    settings = SettingsRepository(os.path.join("data", "settings.db"))

    client = create_client(settings)
    client.run(os.environ["DISCORD_BOT_TOKEN"])
