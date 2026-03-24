import os
import re
import json
import asyncio
import logging
import discord
from discord import app_commands
from typing import Union
from dotenv import load_dotenv
from datetime import datetime, timezone, timedelta

from log_watcher import LogWatcher

JST = timezone(timedelta(hours=9))

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
)

load_dotenv()

DISCORD_BOT_TOKEN: str = os.environ["DISCORD_BOT_TOKEN"]

DATA_DIR = "data"
os.makedirs(DATA_DIR, exist_ok=True)
SETTINGS_FILE = os.path.join(DATA_DIR, "settings.json")

SPECIAL_BALL_PATTERN = re.compile(
    r"\[DailySpecialBallManager\].*Special balls are ([\d,\s]+)"
)

BALL_NAMES = {
    1: ("橙", "連チャンした後に別のボールに変わるよ"),
    2: ("緑", "ルーレットフィーバーを回すよ"),
    4: ("ピンク", "ボールをさらに増やすよ"),
    5: ("紫", "プッシャー台を平らにしてボールを増やすよ"),
    6: ("白", "多めにメダルを出すよ"),
    7: ("水色", "シャルべチャンスの抽選ボールを出すよ"),
    8: ("灰桜", "シャルべチャンスで1マス進むよ"),
    9: ("群青", "シャルベJPプログレッシブカウンターを増やすよ"),
    10: ("赤", "連チャンした後に別のボールに変わるよ"),
    11: ("クリーム", "シャルべチャンスの倍率を上げるよ"),
    12: ("ミント", "強い抽選ボールを出すことがあるよ"),
    13: ("ピーチ", "SPメダルを出すよ"),
    14: ("星空", "時々同じボールを2つ出すよ"),
    15: ("草原", "パレッタチャンスの倍率を上げるよ"),
    16: ("黒", "たまにレインボーシャルべを出すよ"),
    17: ("黄", "パレッタチャンスを早く回すよ"),
    18: ("ラベンダー", "パレッタJPプログレッシブカウンターを増やすよ"),
    19: ("ライム", "メダル投入量を一時的に増やすよ"),
}


def load_settings() -> dict[str, int]:
    if os.path.exists(SETTINGS_FILE):
        try:
            with open(SETTINGS_FILE, "r", encoding="utf-8") as f:
                return json.load(f)
        except (json.JSONDecodeError, OSError):
            pass
    return {}


def save_settings(settings: dict[str, int]) -> None:
    with open(SETTINGS_FILE, "w", encoding="utf-8") as f:
        json.dump(settings, f, ensure_ascii=False, indent=2)


class BotClient(discord.Client):
    def __init__(self, **kwargs):
        super().__init__(**kwargs)
        self.tree = app_commands.CommandTree(self)
        self.settings: dict[str, int] = load_settings()

    async def setup_hook(self) -> None:
        await self.tree.sync()

        watcher = LogWatcher(
            log_dir=os.path.join(
                os.path.expanduser("~"), "AppData", "LocalLow", "VRChat", "VRChat"
            ),
            on_line=self._on_line,
            read_from_end=True,
        )
        asyncio.create_task(watcher.run(), name="log_watcher")

    async def on_ready(self) -> None:
        logging.info("Discord bot ready: %s (id=%s)", self.user, self.user.id)
        logging.info("Loaded settings: %s", self.settings)
        logging.info("Guilds: %s", [g.name for g in self.guilds])

    async def _on_line(self, path: str, line: str) -> None:
        m = SPECIAL_BALL_PATTERN.search(line)
        if not m:
            return

        logging.info("Pattern matched: %r -> group(1)=%r", line, m.group(1))

        balls = [
            BALL_NAMES.get(int(n.strip()), (n.strip(), ""))
            for n in m.group(1).split(",")
            if n.strip()
        ]
        logging.info("Parsed balls: %s", balls)

        if not self.settings:
            logging.warning("No channels configured. Use /setchannel to set one.")
            return

        now = datetime.now(JST)
        if not (
            now.hour == 9 and now.minute <= 5 or now.hour == 8 and now.minute >= 55
        ):
            return

        lines = [f"# {now.month}/{now.day}"]
        for name, desc in balls:
            lines.append(f"## {name}")
            if desc:
                lines.append(desc)
        message = "\n".join(lines)

        for guild_id, channel_id in self.settings.items():
            logging.info("Looking up channel %d (guild=%s)...", channel_id, guild_id)
            channel = self.get_channel(channel_id)
            if channel is None:
                try:
                    channel = await self.fetch_channel(channel_id)
                except discord.NotFound:
                    logging.warning(
                        "Channel %d not found (guild=%s)", channel_id, guild_id
                    )
                    continue
                except discord.Forbidden:
                    logging.warning(
                        "No permission to access channel %d (guild=%s)",
                        channel_id,
                        guild_id,
                    )
                    continue
            logging.info("Sending to channel %s (%d)...", channel.name, channel_id)
            await channel.send(message)
            logging.info(
                "Sent special balls %s to channel %d (guild=%s)",
                [b[0] for b in balls],
                channel_id,
                guild_id,
            )


client = BotClient(intents=discord.Intents.default())


@client.tree.command(name="setchannel", description="通知を送るチャンネル設定します")
@app_commands.describe(channel="テキストチャンネルまたはフォーラムの投稿")
async def set_channel(
    interaction: discord.Interaction,
    channel: Union[discord.TextChannel, discord.Thread],
):
    client.settings[str(interaction.guild_id)] = channel.id
    save_settings(client.settings)
    await interaction.response.send_message(f"{channel.mention} に通知を送るよ！")
    logging.info("Set channel %d for guild %s", channel.id, interaction.guild_id)


@client.tree.command(name="unsetchannel", description="通知を解除します")
async def unset_channel(interaction: discord.Interaction):
    guild_id = str(interaction.guild_id)
    if guild_id in client.settings:
        del client.settings[guild_id]
        save_settings(client.settings)
        await interaction.response.send_message("通知チャンネルの設定を解除したよ！")
    else:
        await interaction.response.send_message("まだチャンネルが設定されていないよ。")


if __name__ == "__main__":
    client.run(DISCORD_BOT_TOKEN)
