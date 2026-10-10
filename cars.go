package main

import "github.com/JMThomas00/Concord/sdk/arcade"

// carModel is one car: 12 x 4 pixels facing right. C is the paint, c its
// darker shade, P/p the grapes driving, k tyres, y lights, B wood, G steel,
// W white, R pink, b hover glow, s mud, g leaf.
type carModel struct {
	arcade.Unlockable
	Short string // fits a 12-column showroom slot
	Blurb string
	Px    []string
}

var cars = []carModel{
	{arcade.Unlockable{ID: "kart", Name: "GO-KART", Tier: arcade.Starter}, "GO-KART", "Small, loud, honest.", []string{"....pP......", "...PPPP.....", "CCCCCCCCCCC.", ".kk......kk."}},
	{arcade.Unlockable{ID: "stock", Name: "STOCK CAR", Tier: arcade.Starter}, "STOCK CAR", "Turns left beautifully.", []string{"...CCCCC....", "..CcPpcCC...", "CCCCCCCCCCCy", ".kk.....kk.."}},
	{arcade.Unlockable{ID: "hatch", Name: "HATCHBACK", Tier: arcade.Starter}, "HATCHBACK", "Room for the groceries.", []string{"....CCCCC...", "...CPpCCCC..", ".CCCCCCCCCCy", "..kk....kk.."}},
	{arcade.Unlockable{ID: "f1", Name: "F1 RACER", Tier: arcade.Common}, "F1 RACER", "Downforce for days.", []string{"C....Pp.....", "CC..CPPCC...", "CCCCCCCCCCCC", ".kkk....kkk."}},
	{arcade.Unlockable{ID: "drag", Name: "DRAGSTER", Tier: arcade.Common}, "DRAGSTER", "A quarter mile at a time.", []string{"CC.Pp.......", "kkCPPCCCCC..", "kkCCCCCCCCCC", "kk.......k.."}},
	{arcade.Unlockable{ID: "pickup", Name: "PICKUP", Tier: arcade.Common}, "PICKUP", "Hauls a whole harvest.", []string{".......CCC..", "C.....CPpC..", "CCCCCCCCCCCy", ".kk.....kk.."}},
	{arcade.Unlockable{ID: "roadster", Name: "VINTAGE ROADSTER", Tier: arcade.Common}, "ROADSTER", "Est. 1932. Still runs.", []string{"......Pp....", "k..CCCPPCC..", "kCCCCCCCCCCy", ".kkk....kkk."}},
	{arcade.Unlockable{ID: "rally", Name: "RALLY CAR", Tier: arcade.Common}, "RALLY CAR", "Mud is a feature.", []string{"...CyyyC....", "..CPpGCCCC..", "CCCsCCsCCCCy", ".kk.....kk.."}},
	{arcade.Unlockable{ID: "moto", Name: "MOTORBIKE", Tier: arcade.Common}, "MOTORBIKE", "Grapes, helmet on.", []string{"......Pp....", "....CCPPC...", "..CCCCCCCC..", ".kkk....kkk."}},
	{arcade.Unlockable{ID: "monster", Name: "MONSTER TRUCK", Tier: arcade.Rare}, "MONSTER", "Crushes cars. Makes wine.", []string{"..CCCCCPpC..", ".CCCCCCCCCy.", "kkk.....kkk.", "kkk.....kkk."}},
	{arcade.Unlockable{ID: "hover", Name: "HOVER CAR", Tier: arcade.Rare}, "HOVER CAR", "Wheels are so last year.", []string{"....CPpC....", ".CCCCCCCCCC.", "CCCCCCCCCCCy", "..b.b..b.b.."}},
	{arcade.Unlockable{ID: "icecream", Name: "ICE CREAM VAN", Tier: arcade.Rare}, "ICE CREAM", "Plays a tune. Never stops.", []string{"....RR......", "CCCCCCCPpCC.", "CCCCCCCCCCCy", ".kk......kk."}},
	{arcade.Unlockable{ID: "tanker", Name: "WINE TANKER", Tier: arcade.Rare}, "TANKER", "Contents: Merlot. Do not shake.", []string{".GGGGGG.CCC.", "GGGRRGG.CPp.", "CCCCCCCCCCCy", ".kk.kk...kk."}},
	{arcade.Unlockable{ID: "barrel", Name: "WINE BARREL", Tier: arcade.Legendary}, "BARREL", "Aged in oak. Driven in anger.", []string{"...PpP......", ".BCBBBBCB...", ".BCBBBBCB...", "..k.....k..."}},
	{arcade.Unlockable{ID: "cart", Name: "SHOPPING CART", Tier: arcade.Legendary}, "CART", "One wheel squeaks. Always.", []string{"G.GPpPpG....", ".GCCCCCCG...", "..GGGGGGG...", "..k....k...."}},
	{arcade.Unlockable{ID: "tub", Name: "BATHTUB", Tier: arcade.Legendary}, "BATHTUB", "Hot water not included.", []string{".W..PpP..W..", "CCCCCCCCCCC.", ".CCCCCCCCC..", "..k.....k..."}},
	{arcade.Unlockable{ID: "couch", Name: "COUCH", Tier: arcade.Legendary}, "COUCH", "Comfiest DNF in the league.", []string{"CCC.PpP.....", "CCCPPPPCCCC.", "CCCCCCCCCCC.", ".k.......k.."}},
	{arcade.Unlockable{ID: "grape", Name: "GIANT GRAPE", Tier: arcade.Legendary}, "GIANT GRAPE", "Grapes all the way down.", []string{"...gPPPP....", "..PPPpPPP...", "..PCCCCCP...", "...PPPP....."}},
}

// carItems are the cars as unlockables, for arcade.Rewards.
var carItems = func() []arcade.Unlockable {
	out := make([]arcade.Unlockable, len(cars))
	for i, c := range cars {
		out[i] = c.Unlockable
	}
	return out
}()

var carByID = func() map[string]*carModel {
	m := map[string]*carModel{}
	for i := range cars {
		m[cars[i].ID] = &cars[i]
	}
	return m
}()

// carPal colours a car: the paint and its shade, the grapes, and the fixed
// parts.
func carPal(paint string) map[rune]string {
	return map[rune]string{
		'C': paint, 'c': paint + "D", 'P': "purple", 'p': "hi", 'k': "tire", 'y': "yellow",
		'B': "orangeD", 'G': "comment", 'W': "fg", 'R': "pink", 'b': "cyan", 's': "orangeD", 'g': "green",
	}
}

// ghostPal draws a car as a dark silhouette (locked).
var ghostPal = func() map[rune]string {
	m := map[rune]string{}
	for k := range carPal("fg") {
		m[k] = "ghost"
	}
	return m
}()

// drawCar draws a car model at (x, py), scaled, optionally flipped (spun
// out) or as a locked silhouette.
func drawCar(c *arcade.Canvas, id, paint string, x, py, scale int, flip, ghost bool) {
	m := carByID[id]
	if m == nil {
		m = carByID["kart"]
	}
	px := m.Px
	if flip {
		px = arcade.Flip(px)
	}
	pal := carPal(paint)
	if ghost {
		pal = ghostPal
	}
	c.Sprite(x, py, px, pal, scale)
}
