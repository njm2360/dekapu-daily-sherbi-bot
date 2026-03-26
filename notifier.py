import logging
import discord
from datetime import datetime, timezone, timedelta

from ball_parser import Lang, format_balls
from settings import SettingsRepository

JST = timezone(timedelta(hours=9))

logger = logging.getLogger(__name__)

_MONTHS_EN = [
    "January", "February", "March", "April", "May", "June",
    "July", "August", "September", "October", "November", "December",
]


def _build_message(ball_ids: list[int], now: datetime, lang: Lang = Lang.JA) -> str:
    if lang == Lang.EN:
        header = f"# {_MONTHS_EN[now.month - 1]} {now.day}"
    else:
        header = f"# {now.month}/{now.day}"

    lines = [header]
    for ball in format_balls(ball_ids, lang):
        lines.append(f"## {ball.name}")
        if ball.description:
            lines.append(ball.description)
    return "\n".join(lines)


class Notifier:
    def __init__(self, client: discord.Client, settings: SettingsRepository) -> None:
        self._client = client
        self._settings = settings

    async def notify(self, ball_ids: list[int]) -> None:
        now = datetime.now(JST)
        for cfg in self._settings.get_all_channels():
            message = _build_message(ball_ids, now, cfg.lang)

            channel = self._client.get_channel(cfg.channel_id)
            if channel is None:
                try:
                    channel = await self._client.fetch_channel(cfg.channel_id)
                except discord.NotFound:
                    logger.warning("Channel %d not found (guild=%s)", cfg.channel_id, cfg.guild_id)
                    continue
                except discord.Forbidden:
                    logger.warning(
                        "No permission to access channel %d (guild=%s)", cfg.channel_id, cfg.guild_id
                    )
                    continue

            role_id = self._settings.get_mention_role(cfg.guild_id)
            content = f"<@&{role_id}>\n{message}" if role_id is not None else message
            await channel.send(content)
            logger.info(
                "Sent special balls %s to channel %d (guild=%s, lang=%s)",
                ball_ids, cfg.channel_id, cfg.guild_id, cfg.lang,
            )
