"""Random name generator for evolved genomes."""

import random

# Adjectives - ~150 evocative, game-appropriate words
ADJECTIVES = [
    # Colors and materials
    "red", "blue", "green", "gold", "silver", "bronze", "copper", "iron",
    "steel", "amber", "jade", "ruby", "onyx", "pearl", "ivory", "ebony",
    "crimson", "azure", "emerald", "scarlet", "violet", "indigo", "cobalt",
    # Light and dark
    "bright", "dark", "shadow", "gleaming", "dim", "radiant", "pale", "vivid",
    "glowing", "shining", "faded", "lustrous", "muted", "stark", "dusky",
    # Size and shape
    "grand", "vast", "twin", "lone", "dual", "triple", "high", "low", "deep",
    "wide", "narrow", "tall", "small", "mighty", "little", "great", "minor",
    # Speed and movement
    "swift", "quick", "rapid", "slow", "fleet", "nimble", "agile", "steady",
    "dashing", "racing", "gliding", "soaring", "leaping", "prowling",
    # Personality
    "bold", "brave", "fierce", "gentle", "wild", "calm", "keen", "sly",
    "cunning", "wise", "clever", "proud", "humble", "noble", "rogue", "sage",
    "grim", "fair", "just", "true", "loyal", "feral", "savage", "tame",
    # Elements
    "frost", "flame", "storm", "thunder", "lightning", "wind", "stone", "earth",
    "water", "fire", "ice", "smoke", "ash", "dust", "mist", "cloud",
    # Time and age
    "ancient", "elder", "young", "prime", "first", "last", "new", "old",
    "eternal", "fleeting", "endless", "timeless", "primal", "modern",
    # Celestial
    "moon", "sun", "star", "void", "cosmic", "astral", "lunar", "solar",
    "stellar", "night", "dawn", "dusk", "twilight", "midnight",
    # Abstract
    "royal", "mystic", "arcane", "sacred", "cursed", "blessed", "hidden",
    "secret", "silent", "loud", "quiet", "still", "sharp", "blunt",
    # More descriptors
    "northern", "southern", "eastern", "western", "inner", "outer", "upper",
    "woven", "forged", "carved", "painted", "gilded", "rusted", "polished",
]

# Nouns - ~150 card/game themed words
NOUNS = [
    # Card terms
    "ace", "king", "queen", "jack", "joker", "trump", "trick", "hand",
    "deck", "deal", "draw", "fold", "bid", "ante", "pot", "stake",
    "club", "spade", "heart", "diamond", "suit", "rank", "card", "shuffle",
    "wager", "bet", "call", "raise", "bluff", "tell", "river", "flop",
    # Animals
    "wolf", "hawk", "fox", "bear", "lion", "tiger", "eagle", "raven",
    "snake", "dragon", "phoenix", "griffin", "stag", "hound", "falcon",
    "viper", "panther", "leopard", "owl", "crow", "shark", "whale",
    "stallion", "mare", "bull", "ram", "boar", "elk", "lynx", "badger",
    # Weapons and tools
    "blade", "sword", "dagger", "axe", "spear", "bow", "arrow", "shield",
    "hammer", "lance", "mace", "staff", "wand", "pike", "scythe", "whip",
    # Structures
    "crown", "throne", "tower", "gate", "forge", "vault", "keep", "castle",
    "citadel", "fortress", "temple", "shrine", "altar", "bridge", "wall",
    "haven", "sanctum", "bastion", "holdfast", "spire", "dome", "arch",
    # Actions/concepts
    "gambit", "feint", "strike", "guard", "clash", "duel", "match", "bout",
    "raid", "siege", "quest", "hunt", "chase", "flight", "march", "charge",
    "pact", "oath", "vow", "bond", "rite", "trial", "test", "challenge",
    # Objects
    "ring", "gem", "orb", "tome", "scroll", "rune", "sigil", "glyph",
    "chalice", "goblet", "flask", "vial", "coin", "token", "key", "lock",
    "mask", "helm", "cloak", "banner", "crest", "seal", "mark", "brand",
    # Nature
    "peak", "vale", "glen", "ridge", "cliff", "crag", "stone", "rock",
    "river", "lake", "sea", "ocean", "wave", "tide", "shore", "coast",
    "forest", "grove", "wood", "marsh", "moor", "plain", "field", "meadow",
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


def generate_unique_name(existing_names: set = None, max_attempts: int = 1000) -> str:
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
