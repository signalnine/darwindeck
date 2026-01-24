<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import Hand from '$lib/components/Hand.svelte';
	import Card from '$lib/components/Card.svelte';
	import {
		startGame,
		applyMove,
		rateGame,
		fetchGame,
		parseCard,
		type GameState,
		type Move
	} from '$lib/api';

	// Route param
	const gameId = $derived($page.params.id);

	// Game state
	let sessionId = $state<string | null>(null);
	let gameName = $state<string>('');
	let gameState = $state<GameState | null>(null);
	let selectedCardIndex = $state<number | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let submitting = $state(false);

	// Game info state
	let showRules = $state(false);
	let gameRules = $state<string | null>(null);

	// AI move display
	let lastAiMoves = $state<Move[]>([]);

	// Rating state
	let showRating = $state(false);
	let rating = $state(3);
	let ratingComment = $state('');
	let ratingSubmitted = $state(false);

	// Parse genome to generate simple rules text
	function parseGenomeRules(genomeJson: string): string {
		try {
			const genome = JSON.parse(genomeJson);
			const lines: string[] = [];

			// Player count
			lines.push(`**Players:** ${genome.player_count || 2}`);

			// Cards per player
			if (genome.setup?.cards_per_player) {
				lines.push(`**Cards dealt:** ${genome.setup.cards_per_player} per player`);
			}

			// Game type from phases
			const phases = genome.turn_structure?.phases || [];
			const phaseTypes = [...new Set(phases.map((p: any) => p.type))];

			if (genome.turn_structure?.is_trick_based) {
				lines.push(`**Type:** Trick-taking game`);
				if (genome.turn_structure.tricks_per_hand) {
					lines.push(`**Tricks per hand:** ${genome.turn_structure.tricks_per_hand}`);
				}
			} else if (phaseTypes.includes('BettingPhase')) {
				lines.push(`**Type:** Betting/Poker-style game`);
			} else if (phaseTypes.includes('PlayPhase')) {
				lines.push(`**Type:** Card playing game`);
			}

			// Win conditions
			const winConditions = genome.win_conditions || [];
			if (winConditions.length > 0) {
				const winTexts = winConditions.map((wc: any) => {
					switch (wc.type) {
						case 'low_score': return `Lowest score wins (under ${wc.threshold} points)`;
						case 'high_score': return `Highest score wins (reach ${wc.threshold} points)`;
						case 'empty_hand': return 'First to empty hand wins';
						case 'all_hands_empty': return 'Game ends when all cards played';
						case 'most_captured': return 'Most captured cards wins';
						default: return wc.type;
					}
				});
				lines.push(`**Win condition:** ${winTexts.join(' or ')}`);
			}

			// Betting info
			if (genome.setup?.starting_chips > 0) {
				lines.push(`**Starting chips:** ${genome.setup.starting_chips}`);
			}

			// Phase details
			if (phases.length > 0) {
				lines.push('');
				lines.push('**Turn phases:**');
				phases.forEach((phase: any, i: number) => {
					if (phase.type === 'TrickPhase') {
						let desc = `${i + 1}. Play a card to the trick`;
						if (phase.lead_suit_required) desc += ' (must follow suit)';
						if (phase.trump_suit) desc += ` (${phase.trump_suit} is trump)`;
						if (phase.high_card_wins === false) desc += ' (low card wins)';
						lines.push(desc);
					} else if (phase.type === 'DrawPhase') {
						lines.push(`${i + 1}. Draw cards`);
					} else if (phase.type === 'PlayPhase') {
						lines.push(`${i + 1}. Play a card`);
					} else if (phase.type === 'BettingPhase') {
						lines.push(`${i + 1}. Betting round`);
					}
				});
			}

			return lines.join('\n');
		} catch {
			return 'Unable to parse game rules.';
		}
	}

	// Derived state
	const isGameOver = $derived(gameState?.winner !== null && gameState?.winner !== undefined);
	const isPlayerTurn = $derived(gameState?.active_player === 0 && !isGameOver);
	const playerHand = $derived(gameState?.hands[0] ?? []);
	const opponentHandSize = $derived(gameState?.hands[1]?.length ?? 0);
	const legalMoves = $derived(gameState?.legal_moves ?? []);

	async function initGame() {
		loading = true;
		error = null;
		lastAiMoves = [];
		const currentGameId = gameId;
		if (!currentGameId) {
			error = 'No game ID provided';
			loading = false;
			return;
		}
		try {
			// Fetch game details for rules (in parallel with starting game)
			const [gameResponse, gameDetails] = await Promise.all([
				startGame(currentGameId, 'greedy'),
				fetchGame(currentGameId).catch(() => null)
			]);

			sessionId = gameResponse.session_id;
			gameName = gameResponse.genome_name;
			gameState = gameResponse.state;

			// Parse rules from genome
			if (gameDetails?.genome_json) {
				gameRules = parseGenomeRules(gameDetails.genome_json);
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to start game';
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		initGame();
	});

	async function handleMoveSelect(move: Move) {
		if (!sessionId || !gameState || submitting) return;

		submitting = true;
		error = null;

		try {
			const response = await applyMove(sessionId, move, gameState.version);
			gameState = { ...response.state, version: response.version };
			selectedCardIndex = null;

			// Track AI moves for display
			if (response.ai_moves && response.ai_moves.length > 0) {
				lastAiMoves = response.ai_moves;
			} else {
				lastAiMoves = [];
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to apply move';
		} finally {
			submitting = false;
		}
	}

	function handleCardSelect(index: number) {
		if (!isPlayerTurn) return;
		selectedCardIndex = selectedCardIndex === index ? null : index;
	}

	function getMoveDescription(move: Move): string {
		// For play moves, generate label from hand card to ensure correct encoding
		// (Go worker uses different card encoding than frontend)
		if (move.type === 'play' && move.card_index !== undefined && move.card_index >= 0) {
			const cardInt = playerHand[move.card_index];
			if (cardInt !== undefined) {
				const card = parseCard(cardInt);
				const ranks = ['', 'A', '2', '3', '4', '5', '6', '7', '8', '9', '10', 'J', 'Q', 'K'];
				const suits = ['\u2663', '\u2666', '\u2665', '\u2660'];
				return `Play ${ranks[card.rank]}${suits[card.suit]}`;
			}
		}
		// For non-play moves, use the label from the worker
		if (move.label && typeof move.label === 'string') {
			return move.label;
		}
		// Fallback descriptions
		if (move.type === 'draw') return 'Draw a card';
		if (move.type === 'pass') return 'Pass';
		if (move.type === 'check') return 'Check';
		if (move.type === 'bet') return 'Bet';
		if (move.type === 'call') return 'Call';
		if (move.type === 'raise') return 'Raise';
		if (move.type === 'fold') return 'Fold';
		if (move.type === 'all_in') return 'All In';
		return move.type;
	}

	// Filter moves for selected card
	const filteredMoves = $derived(() => {
		if (selectedCardIndex === null) {
			// Show non-card moves when no card selected (card_index is undefined or -1)
			return legalMoves.filter((m) => m.card_index === undefined || m.card_index === -1);
		}
		// Show moves for selected card
		return legalMoves.filter((m) => m.card_index === selectedCardIndex);
	});

	async function handleRatingSubmit() {
		if (ratingSubmitted) return;
		const currentGameId = gameId;
		if (!currentGameId) {
			error = 'No game ID for rating';
			return;
		}
		try {
			await rateGame(currentGameId, rating, ratingComment || undefined);
			ratingSubmitted = true;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to submit rating';
		}
	}

	function getWinnerText(): string {
		if (!gameState || gameState.winner === null) return '';
		return gameState.winner === 0 ? 'You won!' : 'AI won!';
	}
</script>

<svelte:head>
	<title>{gameName || 'Game'} - DarwinDeck</title>
</svelte:head>

<main>
	<nav>
		<a href="/">&larr; Back to Games</a>
	</nav>

	{#if loading}
		<div class="loading">
			<p>Starting game...</p>
		</div>
	{:else if error && !gameState}
		<div class="error">
			<p>Error: {error}</p>
			<button onclick={() => initGame()}>Retry</button>
		</div>
	{:else if gameState}
		<div class="game-container">
			<header class="game-header">
				<h1>{gameName}</h1>
				<div class="game-info">
					<span class="turn">Turn {gameState.turn}</span>
					<span class="phase">{gameState.phase}</span>
					{#if gameState.pot}
						<span class="pot">Pot: {gameState.pot}</span>
					{/if}
					<button class="rules-btn" onclick={() => showRules = !showRules}>
						{showRules ? 'Hide Rules' : 'How to Play'}
					</button>
				</div>
			</header>

			{#if showRules && gameRules}
				<div class="rules-panel">
					<h3>Game Rules</h3>
					<div class="rules-content">
						{#each gameRules.split('\n') as line}
							{#if line.startsWith('**') && line.includes(':**')}
								<p><strong>{line.replace(/\*\*/g, '').split(':')[0]}:</strong>{line.split(':').slice(1).join(':').replace(/\*\*/g, '')}</p>
							{:else if line.trim()}
								<p>{line}</p>
							{/if}
						{/each}
					</div>
				</div>
			{/if}

			{#if error}
				<div class="error-banner">
					{error}
					<button onclick={() => (error = null)}>Dismiss</button>
				</div>
			{/if}

			<!-- Opponent area -->
			<section class="opponent-area">
				<div class="opponent-info">
					<span class="player-label">AI Opponent</span>
					<span class="card-count">{opponentHandSize} cards</span>
					{#if gameState.chips && gameState.chips[1] !== undefined}
						<span class="chips">Chips: {gameState.chips[1]}</span>
					{/if}
					{#if gameState.scores && gameState.scores[1] !== undefined}
						<span class="score">Score: {gameState.scores[1]}</span>
					{/if}
				</div>
				<Hand cards={Array(opponentHandSize).fill(0)} faceDown small label="" />

				{#if lastAiMoves.length > 0}
					<div class="ai-moves-display">
						<span class="ai-moves-label">AI played:</span>
						{#each lastAiMoves as aiMove}
							<span class="ai-move-tag">{aiMove.label || aiMove.type}</span>
						{/each}
					</div>
				{/if}
			</section>

			<!-- Tableau / Center area -->
			{#if gameState.tableau && gameState.tableau.some((t) => t.length > 0)}
				<section class="tableau-area">
					<h3>Tableau</h3>
					<div class="tableau-piles">
						{#each gameState.tableau as pile, i}
							{#if pile.length > 0}
								<div class="pile">
									<span class="pile-label">Pile {i + 1}</span>
									<Hand cards={pile} small />
								</div>
							{/if}
						{/each}
					</div>
				</section>
			{/if}

			<!-- Player area -->
			<section class="player-area">
				<div class="player-info">
					<span class="player-label">Your Hand</span>
					{#if gameState.chips && gameState.chips[0] !== undefined}
						<span class="chips">Chips: {gameState.chips[0]}</span>
					{/if}
					{#if gameState.scores && gameState.scores[0] !== undefined}
						<span class="score">Score: {gameState.scores[0]}</span>
					{/if}
				</div>
				<Hand
					cards={playerHand}
					selectable={isPlayerTurn && !submitting}
					selectedIndex={selectedCardIndex}
					onselect={(index) => handleCardSelect(index)}
				/>
			</section>

			<!-- Actions area -->
			<section class="actions-area">
				{#if isGameOver}
					<div class="game-over">
						<h2>{getWinnerText()}</h2>
						<div class="game-over-actions">
							<button class="btn-primary" onclick={() => initGame()}>Play Again</button>
							<button class="btn-secondary" onclick={() => (showRating = true)}>Rate Game</button>
						</div>
					</div>
				{:else if isPlayerTurn}
					<div class="move-selector">
						<p class="turn-indicator">Your turn - select a move</p>
						<div class="moves-list">
							{#each filteredMoves() as move, i}
								<button
									class="move-btn"
									onclick={() => handleMoveSelect(move)}
									disabled={submitting}
								>
									{getMoveDescription(move)}
								</button>
							{/each}
							{#if filteredMoves().length === 0 && selectedCardIndex !== null}
								<p class="no-moves">No moves available for this card</p>
							{/if}
						</div>
					</div>
				{:else}
					<div class="waiting">
						<p>Waiting for AI...</p>
					</div>
				{/if}
			</section>
		</div>

		<!-- Rating modal -->
		{#if showRating}
			<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
			<div
				class="modal-overlay"
				onclick={() => (showRating = false)}
				role="presentation"
			>
				<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
				<div class="modal" role="dialog" aria-modal="true" tabindex="-1" onclick={(e) => e.stopPropagation()}>
					<h2>Rate this Game</h2>
					{#if ratingSubmitted}
						<p class="success">Thanks for your feedback!</p>
						<button onclick={() => (showRating = false)}>Close</button>
					{:else}
						<div class="rating-stars">
							{#each [1, 2, 3, 4, 5] as star}
								<button
									class="star"
									class:active={rating >= star}
									onclick={() => (rating = star)}
								>
									{rating >= star ? '\u2605' : '\u2606'}
								</button>
							{/each}
						</div>
						<textarea
							bind:value={ratingComment}
							placeholder="Optional: Share your thoughts about this game..."
							rows="3"
						></textarea>
						<div class="modal-actions">
							<button onclick={() => handleRatingSubmit()}>Submit</button>
							<button class="btn-secondary" onclick={() => (showRating = false)}>Cancel</button>
						</div>
					{/if}
				</div>
			</div>
		{/if}
	{/if}
</main>

<style>
	main {
		max-width: 900px;
		margin: 0 auto;
		padding: 20px;
		min-height: 100vh;
	}

	nav {
		margin-bottom: 20px;
	}

	nav a {
		color: #3498db;
		text-decoration: none;
	}

	nav a:hover {
		text-decoration: underline;
	}

	.loading,
	.error {
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

	.error-banner {
		background: #fee;
		color: #c00;
		padding: 12px;
		border-radius: 4px;
		margin-bottom: 16px;
		display: flex;
		justify-content: space-between;
		align-items: center;
	}

	.error-banner button {
		background: none;
		border: 1px solid #c00;
		color: #c00;
		padding: 4px 8px;
		border-radius: 4px;
		cursor: pointer;
	}

	.game-container {
		display: flex;
		flex-direction: column;
		gap: 20px;
	}

	.game-header {
		text-align: center;
		padding-bottom: 16px;
		border-bottom: 1px solid #eee;
	}

	.game-header h1 {
		margin: 0 0 8px 0;
		color: #2c3e50;
	}

	.game-info {
		display: flex;
		gap: 16px;
		justify-content: center;
		align-items: center;
		color: #666;
		flex-wrap: wrap;
	}

	.game-info span {
		padding: 4px 12px;
		background: #f0f0f0;
		border-radius: 12px;
		font-size: 0.9em;
	}

	.rules-btn {
		padding: 4px 12px;
		background: #3498db;
		color: white;
		border: none;
		border-radius: 12px;
		cursor: pointer;
		font-size: 0.9em;
	}

	.rules-btn:hover {
		background: #2980b9;
	}

	.rules-panel {
		background: #f8f9fa;
		border: 1px solid #e0e0e0;
		border-radius: 8px;
		padding: 16px;
		margin-bottom: 8px;
	}

	.rules-panel h3 {
		margin: 0 0 12px 0;
		color: #2c3e50;
		font-size: 1.1em;
	}

	.rules-content p {
		margin: 4px 0;
		font-size: 0.95em;
		line-height: 1.5;
		color: #333;
	}

	.rules-content strong {
		color: #2c3e50;
		font-size: 0.9em;
	}

	.opponent-area,
	.player-area {
		padding: 16px;
		background: #f8f9fa;
		border-radius: 12px;
	}

	.opponent-info,
	.player-info {
		display: flex;
		gap: 12px;
		align-items: center;
		margin-bottom: 12px;
		flex-wrap: wrap;
	}

	.player-label {
		font-weight: bold;
		color: #2c3e50;
	}

	.card-count,
	.chips,
	.score {
		font-size: 0.9em;
		color: #666;
		padding: 2px 8px;
		background: #e0e0e0;
		border-radius: 4px;
	}

	.ai-moves-display {
		display: flex;
		gap: 8px;
		align-items: center;
		margin-top: 12px;
		padding: 8px 12px;
		background: #e8f4fc;
		border-radius: 6px;
		flex-wrap: wrap;
	}

	.ai-moves-label {
		font-weight: 600;
		color: #2c3e50;
	}

	.ai-move-tag {
		padding: 4px 10px;
		background: #3498db;
		color: white;
		border-radius: 4px;
		font-size: 0.9em;
	}

	.tableau-area {
		padding: 16px;
		background: rgba(0, 100, 0, 0.05);
		border-radius: 12px;
		text-align: center;
	}

	.tableau-area h3 {
		margin: 0 0 12px 0;
		color: #2c3e50;
	}

	.tableau-piles {
		display: flex;
		gap: 16px;
		justify-content: center;
		flex-wrap: wrap;
	}

	.pile {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 4px;
	}

	.pile-label {
		font-size: 0.8em;
		color: #666;
	}

	.actions-area {
		padding: 20px;
		background: #fff;
		border: 1px solid #ddd;
		border-radius: 12px;
		text-align: center;
	}

	.turn-indicator {
		margin: 0 0 16px 0;
		color: #27ae60;
		font-weight: bold;
	}

	.moves-list {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		justify-content: center;
	}

	.move-btn {
		padding: 10px 20px;
		background: #3498db;
		color: white;
		border: none;
		border-radius: 6px;
		cursor: pointer;
		font-size: 1em;
		transition: background 0.2s;
	}

	.move-btn:hover:not(:disabled) {
		background: #2980b9;
	}

	.move-btn:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.no-moves {
		color: #888;
		font-style: italic;
	}

	.waiting {
		color: #888;
	}

	.game-over {
		padding: 20px;
	}

	.game-over h2 {
		font-size: 2em;
		margin: 0 0 20px 0;
		color: #27ae60;
	}

	.game-over-actions {
		display: flex;
		gap: 12px;
		justify-content: center;
	}

	.btn-primary,
	.btn-secondary {
		padding: 12px 24px;
		border: none;
		border-radius: 6px;
		cursor: pointer;
		font-size: 1em;
	}

	.btn-primary {
		background: #27ae60;
		color: white;
	}

	.btn-primary:hover {
		background: #219a52;
	}

	.btn-secondary {
		background: #eee;
		color: #333;
	}

	.btn-secondary:hover {
		background: #ddd;
	}

	/* Modal styles */
	.modal-overlay {
		position: fixed;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		background: rgba(0, 0, 0, 0.5);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 100;
	}

	.modal {
		background: white;
		padding: 24px;
		border-radius: 12px;
		max-width: 400px;
		width: 90%;
	}

	.modal h2 {
		margin: 0 0 16px 0;
	}

	.rating-stars {
		display: flex;
		gap: 8px;
		justify-content: center;
		margin-bottom: 16px;
	}

	.star {
		font-size: 2em;
		background: none;
		border: none;
		cursor: pointer;
		color: #ccc;
		padding: 0;
	}

	.star.active {
		color: #f1c40f;
	}

	.modal textarea {
		width: 100%;
		padding: 12px;
		border: 1px solid #ddd;
		border-radius: 6px;
		font-family: inherit;
		font-size: 1em;
		resize: vertical;
		box-sizing: border-box;
	}

	.modal-actions {
		display: flex;
		gap: 12px;
		justify-content: flex-end;
		margin-top: 16px;
	}

	.modal-actions button {
		padding: 8px 16px;
		border-radius: 4px;
		cursor: pointer;
	}

	.modal-actions button:first-child {
		background: #3498db;
		color: white;
		border: none;
	}

	.success {
		color: #27ae60;
		font-weight: bold;
	}
</style>
