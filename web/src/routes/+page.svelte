<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchGames, type Game } from '$lib/api';

	let games = $state<Game[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let sortBy = $state<'fitness' | 'rating' | 'plays'>('fitness');

	async function loadGames() {
		loading = true;
		error = null;
		try {
			games = await fetchGames(sortBy);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load games';
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		loadGames();
	});

	function handleSortChange(event: Event) {
		const target = event.target as HTMLSelectElement;
		sortBy = target.value as 'fitness' | 'rating' | 'plays';
		loadGames();
	}

	function formatRating(rating: number | null): string {
		if (rating === null) return 'Not rated';
		return `${rating.toFixed(1)} / 5`;
	}

	function formatFitness(fitness: number): string {
		return (fitness * 100).toFixed(1) + '%';
	}
</script>

<svelte:head>
	<title>DarwinDeck - Card Game Evolution</title>
</svelte:head>

<main>
	<header>
		<h1>DarwinDeck</h1>
		<p class="tagline">AI-Evolved Card Games</p>
	</header>

	<div class="controls">
		<label>
			Sort by:
			<select value={sortBy} onchange={handleSortChange}>
				<option value="fitness">Fitness Score</option>
				<option value="rating">Player Rating</option>
				<option value="plays">Play Count</option>
			</select>
		</label>
		<button onclick={() => loadGames()} disabled={loading}>
			{loading ? 'Loading...' : 'Refresh'}
		</button>
	</div>

	{#if error}
		<div class="error">
			<p>Error: {error}</p>
			<button onclick={() => loadGames()}>Retry</button>
		</div>
	{:else if loading}
		<div class="loading">
			<p>Loading games...</p>
		</div>
	{:else if games.length === 0}
		<div class="empty">
			<p>No games available yet.</p>
			<p class="hint">Run evolution to generate some games!</p>
		</div>
	{:else}
		<div class="games-grid">
			{#each games as game (game.id)}
				<a href="/game/{game.id}" class="game-card">
					<h2>{game.summary || game.id}</h2>
					<div class="game-stats">
						<div class="stat">
							<span class="stat-label">Fitness</span>
							<span class="stat-value">{formatFitness(game.fitness)}</span>
						</div>
						<div class="stat">
							<span class="stat-label">Rating</span>
							<span class="stat-value">{formatRating(game.avg_rating)}</span>
						</div>
						<div class="stat">
							<span class="stat-label">Plays</span>
							<span class="stat-value">{game.play_count}</span>
						</div>
					</div>
					<div class="play-button">Play Now</div>
				</a>
			{/each}
		</div>
	{/if}
</main>

<style>
	main {
		max-width: 1200px;
		margin: 0 auto;
		padding: 20px;
	}

	header {
		text-align: center;
		margin-bottom: 40px;
	}

	header h1 {
		font-size: 2.5em;
		margin: 0;
		color: #2c3e50;
	}

	.tagline {
		color: #666;
		font-size: 1.2em;
		margin-top: 8px;
	}

	.controls {
		display: flex;
		gap: 16px;
		align-items: center;
		justify-content: center;
		margin-bottom: 24px;
		flex-wrap: wrap;
	}

	.controls label {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.controls select {
		padding: 8px 12px;
		border: 1px solid #ccc;
		border-radius: 4px;
		font-size: 1em;
	}

	.controls button {
		padding: 8px 16px;
		background: #3498db;
		color: white;
		border: none;
		border-radius: 4px;
		cursor: pointer;
		font-size: 1em;
	}

	.controls button:hover:not(:disabled) {
		background: #2980b9;
	}

	.controls button:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.error,
	.loading,
	.empty {
		text-align: center;
		padding: 40px;
		background: #f8f9fa;
		border-radius: 8px;
	}

	.error {
		background: #fee;
		color: #c00;
	}

	.error button {
		margin-top: 12px;
		padding: 8px 16px;
		background: #c00;
		color: white;
		border: none;
		border-radius: 4px;
		cursor: pointer;
	}

	.hint {
		color: #888;
		font-style: italic;
	}

	.games-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
		gap: 20px;
	}

	.game-card {
		display: block;
		background: white;
		border: 1px solid #ddd;
		border-radius: 12px;
		padding: 20px;
		text-decoration: none;
		color: inherit;
		transition:
			transform 0.2s,
			box-shadow 0.2s;
	}

	.game-card:hover {
		transform: translateY(-4px);
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.15);
	}

	.game-card h2 {
		margin: 0 0 16px 0;
		font-size: 1.3em;
		color: #2c3e50;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.game-stats {
		display: flex;
		gap: 12px;
		margin-bottom: 16px;
	}

	.stat {
		flex: 1;
		text-align: center;
		padding: 8px;
		background: #f8f9fa;
		border-radius: 6px;
	}

	.stat-label {
		display: block;
		font-size: 0.75em;
		color: #888;
		text-transform: uppercase;
		margin-bottom: 4px;
	}

	.stat-value {
		font-weight: bold;
		color: #2c3e50;
	}

	.play-button {
		text-align: center;
		padding: 10px;
		background: #27ae60;
		color: white;
		border-radius: 6px;
		font-weight: bold;
	}

	.game-card:hover .play-button {
		background: #219a52;
	}
</style>
