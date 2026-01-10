"""Core genome schema types and enumerations."""

from enum import Enum
from typing import List, Optional, Union
from dataclasses import dataclass, field


class Rank(Enum):
    """Playing card ranks."""

    ACE = "A"
    TWO = "2"
    THREE = "3"
    FOUR = "4"
    FIVE = "5"
    SIX = "6"
    SEVEN = "7"
    EIGHT = "8"
    NINE = "9"
    TEN = "10"
    JACK = "J"
    QUEEN = "Q"
    KING = "K"


class Suit(Enum):
    """Playing card suits."""

    HEARTS = "H"
    DIAMONDS = "D"
    CLUBS = "C"
    SPADES = "S"


class Location(Enum):
    """Card locations in game."""

    DECK = "deck"
    HAND = "hand"
    DISCARD = "discard"
    TABLEAU = "tableau"


@dataclass
class GameGenome:
    """Placeholder for complete genome structure."""

    schema_version: str = "1.0"
    genome_id: str = ""
    generation: int = 0
