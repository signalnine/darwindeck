"""Random name generator for evolved genomes."""

import random

# Adjectives - evocative, game-appropriate
ADJECTIVES = [
    "swift", "bold", "dark", "wild", "keen", "sly", "grim", "fair",
    "bright", "stark", "grand", "prime", "quick", "sharp", "calm", "fierce",
    "gold", "silver", "iron", "steel", "crimson", "azure", "jade", "amber",
    "royal", "mystic", "ancient", "primal", "feral", "noble", "rogue", "sage",
    "twin", "lone", "dual", "tri", "high", "low", "deep", "vast",
    "frost", "flame", "storm", "shadow", "moon", "sun", "star", "void",
]

# Nouns - card/game themed
NOUNS = [
    "ace", "king", "queen", "jack", "joker", "trump", "trick", "hand",
    "deck", "deal", "draw", "fold", "bid", "ante", "pot", "stake",
    "club", "spade", "heart", "diamond", "suit", "rank", "card", "shuffle",
    "wolf", "hawk", "fox", "bear", "lion", "raven", "snake", "dragon",
    "blade", "crown", "throne", "tower", "gate", "forge", "vault", "keep",
    "gambit", "bluff", "feint", "strike", "guard", "clash", "duel", "match",
]


def generate_name(seed: int = None) -> str:
    """Generate a random two-word game name.

    Args:
        seed: Optional seed for reproducibility

    Returns:
        Name like "SwiftAce" or "CrimsonWolf"
    """
    if seed is not None:
        rng = random.Random(seed)
    else:
        rng = random.Random()

    adj = rng.choice(ADJECTIVES)
    noun = rng.choice(NOUNS)

    # Capitalize each word
    return f"{adj.capitalize()}{noun.capitalize()}"


def generate_unique_name(existing_names: set = None, max_attempts: int = 100) -> str:
    """Generate a unique name not in the existing set.

    Args:
        existing_names: Set of names already used
        max_attempts: Max attempts before adding numeric suffix

    Returns:
        Unique name
    """
    if existing_names is None:
        existing_names = set()

    for _ in range(max_attempts):
        name = generate_name()
        if name not in existing_names:
            return name

    # Fallback: add random suffix
    base = generate_name()
    suffix = random.randint(1000, 9999)
    return f"{base}{suffix}"
