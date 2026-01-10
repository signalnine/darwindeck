"""Generate golden bytecode file for Go testing."""

from pathlib import Path
from cards_evolve.genome.bytecode import BytecodeCompiler
from cards_evolve.genome.examples import create_war_genome


def main():
    """Generate war_genome.bin for Go integration tests."""
    genome = create_war_genome()
    compiler = BytecodeCompiler()
    bytecode = compiler.compile_genome(genome)

    golden_dir = Path(__file__).parent
    output_file = golden_dir / "war_genome.bin"

    with output_file.open("wb") as f:
        f.write(bytecode)

    print(f"Generated {output_file} ({len(bytecode)} bytes)")
    print(f"Version: 1")
    print(f"Player count: {genome.player_count}")
    print(f"Max turns: {genome.max_turns}")


if __name__ == "__main__":
    main()
