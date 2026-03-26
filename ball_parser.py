import re
from dataclasses import dataclass
from enum import Enum

SPECIAL_BALL_PATTERN = re.compile(
    r"\[DailySpecialBallManager\].*Special balls are ([\d,\s]+)"
)


class Lang(str, Enum):
    JA = "ja"
    EN = "en"


@dataclass(frozen=True)
class BallInfo:
    name: str
    description: str


_BALL_NAMES: dict[Lang, dict[int, BallInfo]] = {
    Lang.JA: {
        1: BallInfo("橙", "連チャンした後に別のボールに変わるよ"),
        2: BallInfo("緑", "ルーレットフィーバーを回すよ"),
        4: BallInfo("ピンク", "ボールをさらに増やすよ"),
        5: BallInfo("紫", "プッシャー台を平らにしてボールを増やすよ"),
        6: BallInfo("白", "多めにメダルを出すよ"),
        7: BallInfo("水色", "シャルべチャンスの抽選ボールを出すよ"),
        8: BallInfo("灰桜", "シャルべチャンスで1マス進むよ"),
        9: BallInfo("群青", "シャルベJPプログレッシブカウンターを増やすよ"),
        10: BallInfo("赤", "連チャンした後に別のボールに変わるよ"),
        11: BallInfo("クリーム", "シャルべチャンスの倍率を上げるよ"),
        12: BallInfo("ミント", "強い抽選ボールを出すことがあるよ"),
        13: BallInfo("ピーチ", "SPメダルを出すよ"),
        14: BallInfo("星空", "時々同じボールを2つ出すよ"),
        15: BallInfo("草原", "パレッタチャンスの倍率を上げるよ"),
        16: BallInfo("黒", "たまにレインボーシャルべを出すよ"),
        17: BallInfo("黄", "パレッタチャンスを早く回すよ"),
        18: BallInfo("ラベンダー", "パレッタJPプログレッシブカウンターを増やすよ"),
        19: BallInfo("ライム", "メダル投入量を一時的に増やすよ"),
    },
    Lang.EN: {
        1: BallInfo("Orange", "Chains itself then changes to another color"),
        2: BallInfo("Green", "Spins fever roulettes"),
        4: BallInfo("Pink", "Spawns more balls"),
        5: BallInfo("Purple", "Flattens pusher and spawns balls"),
        6: BallInfo("White", "More payouts"),
        7: BallInfo("Cyan", "Spawns lottery balls for Sherbi Chance"),
        8: BallInfo("Pale Pink", "Advance 1 step at Sherbi Chance"),
        9: BallInfo("Navy", "Raises Sherbi JACKPOT prog. counter"),
        10: BallInfo("Red", "Chains itself then changes to another color"),
        11: BallInfo("Cream", "Increases multiplier for Sherbi Chance"),
        12: BallInfo("Mint", "Has a chance to spawn strong lottery balls"),
        13: BallInfo("Peach", "Spawns a SP medal"),
        14: BallInfo("Starry", "May spawn two duplicated balls"),
        15: BallInfo("Meadow", "Increases multiplier for Paletta Chance"),
        16: BallInfo("Black", "Has small chance to spawn Rainbow Sherbi"),
        17: BallInfo("Yellow", "Speeds up Paletta Chance game"),
        18: BallInfo("Lavender", "Increases Paletta JACKPOT prog. counter"),
        19: BallInfo("Lime", "Temporarily increases medal insertion amount"),
    },
}


def parse_balls(line: str) -> list[int] | None:
    m = SPECIAL_BALL_PATTERN.search(line)
    if not m:
        return None
    return [int(n.strip()) for n in m.group(1).split(",") if n.strip()]


def format_balls(ball_ids: list[int], lang: Lang = Lang.JA) -> list[BallInfo]:
    names = _BALL_NAMES.get(lang, _BALL_NAMES[Lang.JA])
    return [names.get(bid, BallInfo(str(bid), "")) for bid in ball_ids]
