// Package genome provides seed genomes for testing and evolution.
package genome

// Suit constants matching Python's Suit enum.
const (
	SuitHearts   uint8 = 0
	SuitDiamonds uint8 = 1
	SuitClubs    uint8 = 2
	SuitSpades   uint8 = 3
	SuitAny      uint8 = 255
)

// Rank constants matching Python's Rank enum.
const (
	RankTwo   uint8 = 0
	RankThree uint8 = 1
	RankFour  uint8 = 2
	RankFive  uint8 = 3
	RankSix   uint8 = 4
	RankSeven uint8 = 5
	RankEight uint8 = 6
	RankNine  uint8 = 7
	RankTen   uint8 = 8
	RankJack  uint8 = 9
	RankQueen uint8 = 10
	RankKing  uint8 = 11
	RankAce   uint8 = 12
	RankAny   uint8 = 255
)

// CreateWarGenome creates the War card game genome.
// War is a pure luck game with zero meaningful decisions.
func CreateWarGenome() *GameGenome {
	return &GameGenome{
		Name: "War",
		Setup: SetupRules{
			CardsPerPlayer: 26,
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&PlayPhase{
					Target:   LocationTableau,
					MinCards: 1,
					MaxCards: 1,
				},
			},
			MaxTurns:    1000,
			TableauMode: TableauModeWar,
		},
		WinConditions: []WinCondition{
			{Type: WinTypeCaptureAll},
		},
	}
}

// CreateBettingWarGenome creates War with betting mechanics.
func CreateBettingWarGenome() *GameGenome {
	return &GameGenome{
		Name: "Betting War",
		Setup: SetupRules{
			CardsPerPlayer: 26,
			StartingChips:  500,
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&BettingPhase{
					MinBet:    10,
					MaxRaises: 2,
				},
				&PlayPhase{
					Target:   LocationTableau,
					MinCards: 1,
					MaxCards: 1,
				},
			},
			MaxTurns:    1000,
			TableauMode: TableauModeWar,
		},
		WinConditions: []WinCondition{
			{Type: WinTypeCaptureAll},
		},
		HandEval: &HandEvaluation{
			Method: EvalMethodHighCard,
		},
	}
}

// CreateHeartsGenome creates classic 4-player Hearts.
// Must follow suit, Hearts can't be led until broken, lowest score wins.
func CreateHeartsGenome() *GameGenome {
	return &GameGenome{
		Name: "Hearts",
		Setup: SetupRules{
			CardsPerPlayer: 13, // 4 players x 13 = 52
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&TrickPhase{
					LeadSuitRequired: true,
					TrumpSuit:        255, // No trump
					HighCardWins:     true,
					BreakingSuit:     SuitHearts,
				},
			},
			MaxTurns: 200,
		},
		WinConditions: []WinCondition{
			{Type: WinTypeLowScore, Threshold: 100},
			{Type: WinTypeAllHandsEmpty},
		},
		CardScoring: []CardScoringRule{
			{Suit: SuitHearts, Rank: RankAny, Points: 1, Trigger: TriggerTrickWin},
			{Suit: SuitSpades, Rank: RankQueen, Points: 13, Trigger: TriggerTrickWin},
		},
	}
}

// CreateScotchWhistGenome creates Scotch Whist (Catch the Ten).
// Trump-based trick-taking game.
func CreateScotchWhistGenome() *GameGenome {
	return &GameGenome{
		Name: "Scotch Whist",
		Setup: SetupRules{
			CardsPerPlayer: 13,
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&TrickPhase{
					LeadSuitRequired: true,
					TrumpSuit:        SuitSpades,
					HighCardWins:     true,
				},
			},
			MaxTurns: 200,
		},
		WinConditions: []WinCondition{
			{Type: WinTypeMostCaptured},
			{Type: WinTypeAllHandsEmpty},
		},
	}
}

// CreateKnockoutWhistGenome creates Knock-Out Whist.
// Simple elimination trick-taking game.
func CreateKnockoutWhistGenome() *GameGenome {
	return &GameGenome{
		Name: "Knock-Out Whist",
		Setup: SetupRules{
			CardsPerPlayer: 7, // 4 players x 7 = 28 cards
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&TrickPhase{
					LeadSuitRequired: true,
					TrumpSuit:        SuitHearts,
					HighCardWins:     true,
				},
			},
			MaxTurns: 100,
		},
		WinConditions: []WinCondition{
			{Type: WinTypeMostCaptured},
			{Type: WinTypeAllHandsEmpty},
		},
	}
}

// CreateSpadesGenome creates Spades with bidding.
// Classic trick-taking with fixed trump and contract bidding.
func CreateSpadesGenome() *GameGenome {
	return &GameGenome{
		Name: "Spades",
		Setup: SetupRules{
			CardsPerPlayer: 13,
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&BiddingPhase{
					MinBid:                1,
					MaxBid:                13,
					AllowNil:              true,
					PointsPerTrickBid:     10,
					OvertrickPoints:       1,
					FailedContractPenalty: 10,
					NilBonus:              100,
					NilPenalty:            100,
					BagLimit:              10,
					BagPenalty:            100,
				},
				&TrickPhase{
					LeadSuitRequired: true,
					TrumpSuit:        SuitSpades,
					HighCardWins:     true,
					BreakingSuit:     SuitSpades,
				},
			},
			MaxTurns: 200,
		},
		WinConditions: []WinCondition{
			{Type: WinTypeFirstToScore, Threshold: 500},
		},
	}
}

// CreatePartnershipSpadesGenome creates Partnership Spades.
// 4 players in 2 teams (0,2 vs 1,3).
func CreatePartnershipSpadesGenome() *GameGenome {
	return &GameGenome{
		Name: "Partnership Spades",
		Setup: SetupRules{
			CardsPerPlayer: 13,
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&BiddingPhase{
					MinBid:                1,
					MaxBid:                13,
					AllowNil:              true,
					PointsPerTrickBid:     10,
					OvertrickPoints:       1,
					FailedContractPenalty: 10,
					NilBonus:              100,
					NilPenalty:            100,
					BagLimit:              10,
					BagPenalty:            100,
				},
				&TrickPhase{
					LeadSuitRequired: true,
					TrumpSuit:        SuitSpades,
					HighCardWins:     true,
					BreakingSuit:     SuitSpades,
				},
			},
			MaxTurns: 200,
		},
		WinConditions: []WinCondition{
			{Type: WinTypeFirstToScore, Threshold: 500},
		},
		Teams: &TeamConfig{
			Enabled: true,
			Teams:   [][]int{{0, 2}, {1, 3}},
		},
	}
}

// CreateCrazyEightsGenome creates Crazy 8s card game.
// Simplified: match suit/rank of discard, 8s are wild, first empty hand wins.
func CreateCrazyEightsGenome() *GameGenome {
	return &GameGenome{
		Name: "Crazy Eights",
		Setup: SetupRules{
			CardsPerPlayer: 10,
			DealToTableau:  1, // Start with one card in discard
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&DrawPhase{
					Source:    LocationDeck,
					Count:     1,
					Mandatory: false,
				},
				&PlayPhase{
					Target:       LocationDiscard,
					MinCards:     1,
					MaxCards:     4,
					Mandatory:    true,
					PassIfUnable: true,
					// Note: Valid play conditions (match suit/rank/8)
					// handled by interpreter
				},
			},
			MaxTurns: 500,
		},
		WinConditions: []WinCondition{
			{Type: WinTypeEmptyHand},
		},
	}
}

// CreateOldMaidGenome creates Old Maid card game.
// Draw from opponent, discard pairs, avoid the odd card.
func CreateOldMaidGenome() *GameGenome {
	return &GameGenome{
		Name: "Old Maid",
		Setup: SetupRules{
			CardsPerPlayer: 13,
			DealToTableau:  1, // Remove one card to create odd
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&DrawPhase{
					Source:    LocationOpponentHand,
					Count:     1,
					Mandatory: true,
				},
				&DiscardPhase{
					Target:    LocationDiscard,
					Count:     2,
					Mandatory: false, // Only if you have a pair
				},
			},
			MaxTurns: 100,
		},
		WinConditions: []WinCondition{
			{Type: WinTypeEmptyHand},
		},
	}
}

// CreatePresidentGenome creates President/Daifugo game.
// Climbing game where 2 is high, first empty hand wins.
func CreatePresidentGenome() *GameGenome {
	return &GameGenome{
		Name: "President",
		Setup: SetupRules{
			CardsPerPlayer: 13, // 4 players x 13 = 52
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&PlayPhase{
					Target:       LocationTableau,
					MinCards:     1,
					MaxCards:     1,
					Mandatory:    true,
					PassIfUnable: true,
					// Must beat top card - handled by interpreter
				},
			},
			MaxTurns: 300,
		},
		WinConditions: []WinCondition{
			{Type: WinTypeEmptyHand},
		},
	}
}

// CreateFanTanGenome creates Fan Tan / Sevens card game.
// Shedding game with sequential building on tableau.
func CreateFanTanGenome() *GameGenome {
	return &GameGenome{
		Name: "Fan Tan",
		Setup: SetupRules{
			CardsPerPlayer: 10,
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&PlayPhase{
					Target:       LocationTableau,
					MinCards:     1,
					MaxCards:     1,
					Mandatory:    true,
					PassIfUnable: true,
				},
				&DrawPhase{
					Source:    LocationDeck,
					Count:     1,
					Mandatory: false,
				},
			},
			MaxTurns:          150,
			TableauMode:       TableauModeSequence,
			SequenceDirection: SequenceBoth,
		},
		WinConditions: []WinCondition{
			{Type: WinTypeEmptyHand},
		},
	}
}

// CreateUnoStyleGenome creates an Uno-style game with special effects.
// Match suit/rank, special cards have effects (skip, reverse, draw).
func CreateUnoStyleGenome() *GameGenome {
	return &GameGenome{
		Name: "Uno Style",
		Setup: SetupRules{
			CardsPerPlayer: 7,
			DealToTableau:  1, // Start with one card face up
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&PlayPhase{
					Target:       LocationDiscard,
					MinCards:     1,
					MaxCards:     1,
					Mandatory:    false,
					PassIfUnable: true,
				},
				&DrawPhase{
					Source:    LocationDeck,
					Count:     1,
					Mandatory: false,
				},
			},
			MaxTurns: 500,
		},
		WinConditions: []WinCondition{
			{Type: WinTypeEmptyHand},
		},
		Effects: []SpecialEffect{
			{TriggerRank: RankTwo, Effect: EffectDrawTwo, Target: 0, Value: 2},
			{TriggerRank: RankJack, Effect: EffectSkipNext, Target: 0, Value: 1},
			{TriggerRank: RankQueen, Effect: EffectReverse, Target: 2, Value: 1},
		},
	}
}

// CreateGinRummyGenome creates simplified Gin Rummy.
// Draw, meld to tableau, discard - first empty hand wins.
func CreateGinRummyGenome() *GameGenome {
	return &GameGenome{
		Name: "Gin Rummy",
		Setup: SetupRules{
			CardsPerPlayer: 10,
			DealToTableau:  1, // Start discard pile
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&DrawPhase{
					Source:    LocationDeck,
					Count:     1,
					Mandatory: true,
				},
				&PlayPhase{
					Target:    LocationTableau,
					MinCards:  0,
					MaxCards:  10,
					Mandatory: false,
				},
				&DiscardPhase{
					Target:    LocationDiscard,
					Count:     1,
					Mandatory: true,
				},
			},
			MaxTurns: 100,
		},
		WinConditions: []WinCondition{
			{Type: WinTypeEmptyHand},
		},
	}
}

// CreateGoFishGenome creates Go Fish card game.
// Draw, play pairs/sets, first empty hand or highest score wins.
func CreateGoFishGenome() *GameGenome {
	return &GameGenome{
		Name: "Go Fish",
		Setup: SetupRules{
			CardsPerPlayer: 10,
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&DrawPhase{
					Source:    LocationDeck,
					Count:     1,
					Mandatory: true,
				},
				&PlayPhase{
					Target:   LocationTableau,
					MinCards: 2,
					MaxCards: 4,
					// Pairs or sets go to tableau
				},
				&PlayPhase{
					Target:   LocationDiscard,
					MinCards: 4,
					MaxCards: 4,
					// Complete books (4 of a kind) to discard for scoring
				},
				&DiscardPhase{
					Target:    LocationDiscard,
					Count:     1,
					Mandatory: false,
				},
			},
			MaxTurns: 200,
		},
		WinConditions: []WinCondition{
			{Type: WinTypeHighScore, Threshold: 1},
			{Type: WinTypeEmptyHand},
		},
	}
}

// CreateSimplePokerGenome creates Simple Poker with betting.
// 5-card hands, one betting round, best hand wins.
func CreateSimplePokerGenome() *GameGenome {
	return &GameGenome{
		Name: "Simple Poker",
		Setup: SetupRules{
			CardsPerPlayer: 5,
			StartingChips:  1000,
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&BettingPhase{
					MinBet:    10,
					MaxRaises: 3,
				},
			},
			MaxTurns: 10,
		},
		WinConditions: []WinCondition{
			{Type: WinTypeBestHand},
		},
		HandEval: &HandEvaluation{
			Method: EvalMethodPatternMatch,
			Patterns: []HandPattern{
				{Name: "Royal Flush", Priority: 100, RequiredCount: 5, SameSuitCount: 5, SequenceLength: 5, RequiredRanks: []uint8{RankTen, RankJack, RankQueen, RankKing, RankAce}},
				{Name: "Straight Flush", Priority: 90, RequiredCount: 5, SameSuitCount: 5, SequenceLength: 5},
				{Name: "Four of a Kind", Priority: 80, RequiredCount: 5, SameRankGroups: []uint8{4}},
				{Name: "Full House", Priority: 70, RequiredCount: 5, SameRankGroups: []uint8{3, 2}},
				{Name: "Flush", Priority: 60, RequiredCount: 5, SameSuitCount: 5},
				{Name: "Straight", Priority: 50, RequiredCount: 5, SequenceLength: 5, SequenceWrap: true},
				{Name: "Three of a Kind", Priority: 40, RequiredCount: 5, SameRankGroups: []uint8{3}},
				{Name: "Two Pair", Priority: 30, RequiredCount: 5, SameRankGroups: []uint8{2, 2}},
				{Name: "One Pair", Priority: 20, RequiredCount: 5, SameRankGroups: []uint8{2}},
				{Name: "High Card", Priority: 10, RequiredCount: 5},
			},
		},
	}
}

// CreateCheatGenome creates I Doubt It / Cheat / BS card game.
// Play cards with claims, opponents can challenge.
func CreateCheatGenome() *GameGenome {
	return &GameGenome{
		Name: "Cheat",
		Setup: SetupRules{
			CardsPerPlayer: 26,
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&ClaimPhase{},
			},
			MaxTurns: 2000,
		},
		WinConditions: []WinCondition{
			{Type: WinTypeEmptyHand},
		},
	}
}

// CreateScopaGenome creates Scopa (Italian capturing game).
// Play card to capture matching rank from tableau.
func CreateScopaGenome() *GameGenome {
	return &GameGenome{
		Name: "Scopa",
		Setup: SetupRules{
			CardsPerPlayer: 3,
			DealToTableau:  4, // Start with 4 cards on tableau
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&PlayPhase{
					Target:   LocationTableau,
					MinCards: 1,
					MaxCards: 1,
				},
				&DrawPhase{
					Source:    LocationDeck,
					Count:     3,
					Mandatory: true,
					Condition: &Condition{
						OpCode:   0, // Hand size check
						Operator: 0, // EQ
						Value:    0, // Hand is empty
					},
				},
			},
			MaxTurns:    100,
			TableauMode: TableauModeMatchRank,
		},
		WinConditions: []WinCondition{
			{Type: WinTypeMostCaptured},
		},
	}
}

// CreateDrawPokerGenome creates Draw Poker with hand improvement.
// Deal 5, bet, discard/draw, bet again, best hand wins.
func CreateDrawPokerGenome() *GameGenome {
	return &GameGenome{
		Name: "Draw Poker",
		Setup: SetupRules{
			CardsPerPlayer: 5,
			StartingChips:  1000,
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&BettingPhase{
					MinBet:    20,
					MaxRaises: 3,
				},
				&DiscardPhase{
					Target:    LocationDiscard,
					Count:     3,
					Mandatory: false,
				},
				&DrawPhase{
					Source:    LocationDeck,
					Count:     3,
					Mandatory: false,
					Condition: &Condition{
						OpCode:   0, // Hand size check
						Operator: 1, // LT
						Value:    5, // Hand < 5 cards
					},
				},
			},
			MaxTurns: 20,
		},
		WinConditions: []WinCondition{
			{Type: WinTypeBestHand},
		},
		HandEval: &HandEvaluation{
			Method: EvalMethodPatternMatch,
			Patterns: []HandPattern{
				{Name: "Royal Flush", Priority: 100, RequiredCount: 5, SameSuitCount: 5, SequenceLength: 5, RequiredRanks: []uint8{RankTen, RankJack, RankQueen, RankKing, RankAce}},
				{Name: "Straight Flush", Priority: 90, RequiredCount: 5, SameSuitCount: 5, SequenceLength: 5},
				{Name: "Four of a Kind", Priority: 80, RequiredCount: 5, SameRankGroups: []uint8{4}},
				{Name: "Full House", Priority: 70, RequiredCount: 5, SameRankGroups: []uint8{3, 2}},
				{Name: "Flush", Priority: 60, RequiredCount: 5, SameSuitCount: 5},
				{Name: "Straight", Priority: 50, RequiredCount: 5, SequenceLength: 5, SequenceWrap: true},
				{Name: "Three of a Kind", Priority: 40, RequiredCount: 5, SameRankGroups: []uint8{3}},
				{Name: "Two Pair", Priority: 30, RequiredCount: 5, SameRankGroups: []uint8{2, 2}},
				{Name: "One Pair", Priority: 20, RequiredCount: 5, SameRankGroups: []uint8{2}},
				{Name: "High Card", Priority: 10, RequiredCount: 5},
			},
		},
	}
}

// CreateBlackjackGenome creates Blackjack/21 card game.
// Draw to get close to 21 without busting.
func CreateBlackjackGenome() *GameGenome {
	return &GameGenome{
		Name: "Blackjack",
		Setup: SetupRules{
			CardsPerPlayer: 2,
			StartingChips:  500,
		},
		TurnStructure: TurnStructure{
			Phases: []Phase{
				&BettingPhase{
					MinBet:    25,
					MaxRaises: 1,
				},
				&DrawPhase{
					Source:    LocationDeck,
					Count:     1,
					Mandatory: false,
					Condition: &Condition{
						OpCode:   0, // Hand size check
						Operator: 1, // LT
						Value:    5, // Max 5 cards (5-card charlie)
					},
				},
			},
			MaxTurns: 20,
		},
		WinConditions: []WinCondition{
			{Type: WinTypeHighScore, Threshold: 21},
		},
		HandEval: &HandEvaluation{
			Method:        EvalMethodPointTotal,
			TargetValue:   21,
			BustThreshold: 22,
			CardValues: []CardValue{
				{Rank: RankAce, Value: 1, AltValue: 11},
				{Rank: RankTwo, Value: 2},
				{Rank: RankThree, Value: 3},
				{Rank: RankFour, Value: 4},
				{Rank: RankFive, Value: 5},
				{Rank: RankSix, Value: 6},
				{Rank: RankSeven, Value: 7},
				{Rank: RankEight, Value: 8},
				{Rank: RankNine, Value: 9},
				{Rank: RankTen, Value: 10},
				{Rank: RankJack, Value: 10},
				{Rank: RankQueen, Value: 10},
				{Rank: RankKing, Value: 10},
			},
		},
	}
}

// GetSeedGenomes returns all seed genomes for initial population.
// Returns 19 diverse games covering different mechanics:
//   - Luck-based: War, Betting War
//   - Trick-taking: Hearts, Scotch Whist, Knock-Out Whist, Spades, Partnership Spades
//   - Shedding/Matching: Crazy Eights, Old Maid, President, Fan Tan, Uno-style
//   - Set Collection: Gin Rummy, Go Fish
//   - Betting: Simple Poker
//   - Other: Cheat, Scopa, Draw Poker, Blackjack
func GetSeedGenomes() []*GameGenome {
	return []*GameGenome{
		// Luck-based
		CreateWarGenome(),
		CreateBettingWarGenome(),
		// Trick-taking
		CreateHeartsGenome(),
		CreateScotchWhistGenome(),
		CreateKnockoutWhistGenome(),
		CreateSpadesGenome(),
		CreatePartnershipSpadesGenome(),
		// Shedding/Matching
		CreateCrazyEightsGenome(),
		CreateOldMaidGenome(),
		CreatePresidentGenome(),
		CreateFanTanGenome(),
		CreateUnoStyleGenome(),
		// Set Collection
		CreateGinRummyGenome(),
		CreateGoFishGenome(),
		// Betting
		CreateSimplePokerGenome(),
		// Other
		CreateCheatGenome(),
		CreateScopaGenome(),
		CreateDrawPokerGenome(),
		CreateBlackjackGenome(),
	}
}
