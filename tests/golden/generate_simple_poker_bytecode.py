"""Generate golden bytecode file for betting genome Go testing."""

from pathlib import Path
from darwindeck.genome.bytecode import BytecodeCompiler
from darwindeck.genome.examples import create_simple_poker_genome


def main():
    """Generate simple_poker_genome.bin for Go integration tests."""
    genome = create_simple_poker_genome()
    compiler = BytecodeCompiler()
    bytecode = compiler.compile_genome(genome)

    golden_dir = Path(__file__).parent
    output_file = golden_dir / "simple_poker_genome.bin"

    with output_file.open("wb") as f:
        f.write(bytecode)

    print(f"Generated {output_file} ({len(bytecode)} bytes)")
    print(f"Version: 1")
    print(f"Player count: {genome.player_count}")
    print(f"Max turns: {genome.max_turns}")
    print(f"Starting chips: {genome.setup.starting_chips}")
    print(f"Phases: {[type(p).__name__ for p in genome.turn_structure.phases]}")


if __name__ == "__main__":
    main()
