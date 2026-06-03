package app

import "math/rand/v2"

var adjectives = []string{
	"brave", "calm", "cool", "cozy", "cute",
	"dark", "fair", "fast", "glad", "gold",
	"keen", "kind", "lazy", "loud", "mild",
	"neat", "nice", "pale", "pure", "rare",
	"rich", "safe", "slim", "soft", "tall",
	"thin", "true", "warm", "wild", "wise",
}

var nouns = []string{
	"bear", "bird", "colt", "crab", "crow",
	"deer", "dove", "duck", "fawn", "fish",
	"frog", "goat", "gull", "hare", "hawk",
	"lark", "lynx", "mink", "moth", "newt",
	"orca", "puma", "seal", "swan", "toad",
	"vole", "wasp", "wolf", "wren", "yak",
}

func randomName() string {
	return adjectives[rand.IntN(len(adjectives))] + "-" + nouns[rand.IntN(len(nouns))]
}
