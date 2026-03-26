import asyncio
import logging
from datetime import datetime, timezone, timedelta
from pathlib import Path

import discord
from discord import app_commands

from ball_parser import Lang, parse_balls
from log_watcher import LogWatcher
from notifier import Notifier
from settings import SettingsRepository

JST = timezone(timedelta(hours=9))


def _is_notification_time(now: datetime) -> bool:
    return now.hour == 9 and now.minute <= 5 or now.hour == 8 and now.minute >= 55


logger = logging.getLogger(__name__)


class BotClient(discord.Client):
    def __init__(self, settings: SettingsRepository, **kwargs):
        super().__init__(**kwargs)
        self.tree = app_commands.CommandTree(self)
        self.settings = settings
        self._notifier = Notifier(self, settings)

    async def setup_hook(self) -> None:
        await self.tree.sync()

        watcher = LogWatcher(
            log_dir=Path.home() / "AppData" / "LocalLow" / "VRChat" / "VRChat",
            on_line=self._on_line,
            read_from_end=True,
        )
        asyncio.create_task(watcher.run(), name="log_watcher")

    async def on_guild_join(self, guild: discord.Guild) -> None:
        self.settings.ensure_guild(str(guild.id))
        logger.info("Joined guild: %s (id=%s)", guild.name, guild.id)

    async def on_guild_remove(self, guild: discord.Guild) -> None:
        self.settings.remove_guild(str(guild.id))
        logger.info("Left guild: %s (id=%s)", guild.name, guild.id)

    async def on_ready(self) -> None:
        logger.info("Discord bot ready: %s (id=%s)", self.user, self.user.id)

    async def _on_line(self, _: Path, line: str) -> None:
        ball_ids = parse_balls(line)
        if ball_ids is None:
            return
        if not _is_notification_time(datetime.now(JST)):
            return
        logger.info("Parsed balls: %s", ball_ids)
        await self._notifier.notify(ball_ids)


def create_client(settings: SettingsRepository) -> BotClient:
    client = BotClient(settings=settings, intents=discord.Intents.default())

    @client.tree.command(
        name="setchannel", description="このチャンネルに通知を送るよう設定します"
    )
    @app_commands.describe(lang="通知言語（デフォルトは ja）")
    async def set_channel(
        interaction: discord.Interaction,
        lang: Lang = Lang.JA,
    ):
        channel = interaction.channel
        guild_id = str(interaction.guild_id)
        client.settings.ensure_guild(guild_id)
        client.settings.set_channel(guild_id, channel.id, lang)
        label = "日本語" if lang == Lang.JA else "English"
        await interaction.response.send_message(
            f"{channel.mention} に通知を送るよ！（言語: {label}）"
        )
        logger.info(
            "Set channel %d (lang=%s) for guild %s",
            channel.id,
            lang,
            interaction.guild_id,
        )

    @client.tree.command(
        name="unsetchannel", description="このチャンネルの通知設定を解除します"
    )
    async def unset_channel(interaction: discord.Interaction):
        channel = interaction.channel
        if client.settings.unset_channel(str(interaction.guild_id), channel.id):
            await interaction.response.send_message(
                f"{channel.mention} の通知チャンネル設定を解除したよ！"
            )
        else:
            await interaction.response.send_message(
                f"{channel.mention} は通知チャンネルに設定されていないよ。"
            )

    @client.tree.command(
        name="setmentionrole", description="通知時にメンションするロールを設定します"
    )
    @app_commands.describe(role="メンションするロール")
    async def set_mention_role(interaction: discord.Interaction, role: discord.Role):
        client.settings.set_mention_role(str(interaction.guild_id), role.id)
        await interaction.response.send_message(
            f"通知時に {role.mention} をメンションするよ！"
        )
        logger.info("Set mention role %d for guild %s", role.id, interaction.guild_id)

    @client.tree.command(
        name="unsetmentionrole", description="通知時のメンションロール設定を解除します"
    )
    async def unset_mention_role(interaction: discord.Interaction):
        if client.settings.unset_mention_role(str(interaction.guild_id)):
            await interaction.response.send_message(
                "メンションロールの設定を解除したよ！"
            )
        else:
            await interaction.response.send_message(
                "メンションロールは設定されていないよ。"
            )

    return client
