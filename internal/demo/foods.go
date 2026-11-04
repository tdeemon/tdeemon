package demo

import (
	"github.com/zarnautovic/tdeemon/internal/domain/food"
)

// name is a text in each language the app speaks.
type name struct{ en, hr string }

func (n name) in(locale string) string {
	if locale == "hr" {
		return n.hr
	}
	return n.en
}

// The servings the foods name.
var (
	piece = name{"piece", "komad"}
	slice = name{"slice", "šnita"}
	scoop = name{"scoop", "mjerica"}
	tbsp  = name{"tbsp", "žlica"}
)

// demoFood is a generic food of the demo user's: values per 100 g (or
// 100 ml) of the kind a food composition table gives, rounded and made
// up for the demo, of no brand.
type demoFood struct {
	key                  string
	name                 name
	basis                food.Basis
	kcal, p, c, f, fiber float64
	serving              *name
	servingAmount        float64
}

// foods are the demo user's foods, each a reference food.
var foods = []demoFood{
	{key: "oats", name: name{"Rolled oats", "Zobene pahuljice"}, basis: food.Per100g, kcal: 375, p: 13, c: 60, f: 7, fiber: 10},
	{key: "milk", name: name{"Semi-skimmed milk", "Mlijeko, djelomično obrano"}, basis: food.Per100ml, kcal: 47, p: 3.4, c: 4.8, f: 1.6},
	{key: "banana", name: name{"Banana", "Banana"}, basis: food.Per100g, kcal: 89, p: 1.1, c: 20, f: 0.3, fiber: 2.6, serving: &piece, servingAmount: 120},
	{key: "egg", name: name{"Egg", "Jaje"}, basis: food.Per100g, kcal: 143, p: 12.6, c: 0.7, f: 9.5, serving: &piece, servingAmount: 55},
	{key: "bread", name: name{"Wholegrain bread", "Integralni kruh"}, basis: food.Per100g, kcal: 247, p: 13, c: 41, f: 3.4, fiber: 7, serving: &slice, servingAmount: 40},
	{key: "yoghurt", name: name{"Greek yoghurt, 2 % fat", "Grčki jogurt, 2 % m.m."}, basis: food.Per100g, kcal: 73, p: 10, c: 3.6, f: 2},
	{key: "whey", name: name{"Whey protein powder", "Proteinski prah (whey)"}, basis: food.Per100g, kcal: 380, p: 78, c: 8, f: 5, serving: &scoop, servingAmount: 30},
	{key: "blueberries", name: name{"Blueberries", "Borovnice"}, basis: food.Per100g, kcal: 57, p: 0.7, c: 12, f: 0.3, fiber: 2.4},
	{key: "chicken", name: name{"Chicken breast, roasted", "Pileća prsa, pečena"}, basis: food.Per100g, kcal: 165, p: 31, c: 0, f: 3.6},
	{key: "rice", name: name{"White rice, cooked", "Bijela riža, kuhana"}, basis: food.Per100g, kcal: 130, p: 2.7, c: 28.2, f: 0.3},
	{key: "pasta", name: name{"Pasta, cooked", "Tjestenina, kuhana"}, basis: food.Per100g, kcal: 158, p: 5.8, c: 30.9, f: 0.9},
	{key: "oil", name: name{"Olive oil", "Maslinovo ulje"}, basis: food.Per100g, kcal: 884, p: 0, c: 0, f: 100, serving: &tbsp, servingAmount: 10},
	{key: "broccoli", name: name{"Broccoli, steamed", "Brokula, kuhana na pari"}, basis: food.Per100g, kcal: 35, p: 2.4, c: 4.4, f: 0.4, fiber: 3.3},
	{key: "salmon", name: name{"Salmon fillet, baked", "Losos, pečeni file"}, basis: food.Per100g, kcal: 206, p: 22, c: 0, f: 13},
	{key: "potatoes", name: name{"Potatoes, boiled", "Krumpir, kuhani"}, basis: food.Per100g, kcal: 87, p: 1.9, c: 18.5, f: 0.1, fiber: 1.8},
	{key: "beef", name: name{"Beef mince, 10 % fat, raw", "Mljevena junetina, 10 % masti, sirova"}, basis: food.Per100g, kcal: 176, p: 20, c: 0, f: 10},
	{key: "tomato", name: name{"Tomato", "Rajčica"}, basis: food.Per100g, kcal: 18, p: 0.9, c: 2.7, f: 0.2, fiber: 1.2, serving: &piece, servingAmount: 120},
	{key: "salad", name: name{"Mixed salad leaves", "Miješana zelena salata"}, basis: food.Per100g, kcal: 17, p: 1.4, c: 1.9, f: 0.3, fiber: 1.6},
	{key: "apple", name: name{"Apple", "Jabuka"}, basis: food.Per100g, kcal: 52, p: 0.3, c: 11.4, f: 0.2, fiber: 2.4, serving: &piece, servingAmount: 180},
	{key: "almonds", name: name{"Almonds", "Bademi"}, basis: food.Per100g, kcal: 579, p: 21, c: 9.5, f: 50, fiber: 12.5},
	{key: "cottage", name: name{"Cottage cheese", "Svježi kravlji sir"}, basis: food.Per100g, kcal: 98, p: 11, c: 3.4, f: 4.3},
	{key: "tuna", name: name{"Tuna in water, drained", "Tuna u vlastitom soku, ocijeđena"}, basis: food.Per100g, kcal: 116, p: 26, c: 0, f: 1},
	{key: "wrap", name: name{"Tortilla wrap", "Tortilja"}, basis: food.Per100g, kcal: 310, p: 8.5, c: 51, f: 8, fiber: 3, serving: &piece, servingAmount: 60},
	{key: "chocolate", name: name{"Dark chocolate, 70 %", "Tamna čokolada, 70 %"}, basis: food.Per100g, kcal: 598, p: 7.8, c: 36, f: 42.6, fiber: 10.9},
	{key: "peanut", name: name{"Peanut butter", "Maslac od kikirikija"}, basis: food.Per100g, kcal: 588, p: 25, c: 14, f: 50, fiber: 6, serving: &tbsp, servingAmount: 16},
	{key: "cheese", name: name{"Hard cheese", "Tvrdi sir"}, basis: food.Per100g, kcal: 402, p: 25, c: 1.3, f: 33},
	{key: "lentils", name: name{"Red lentils, dry", "Crvena leća, suha"}, basis: food.Per100g, kcal: 358, p: 24.6, c: 52, f: 2.2, fiber: 11},
	{key: "onion", name: name{"Onion", "Luk"}, basis: food.Per100g, kcal: 40, p: 1.1, c: 7.6, f: 0.1, fiber: 1.7},
	{key: "tomatoes", name: name{"Canned chopped tomatoes", "Pelati, sjeckani"}, basis: food.Per100g, kcal: 32, p: 1.6, c: 4.8, f: 0.2, fiber: 1.5},
}

// ingredient is a food of a recipe, in grams.
type ingredient struct {
	food  string
	grams float64
}

// demoRecipe is a dish the demo user cooks: its raw ingredients, the
// weight of the cooked dish and the portions it is divided into.
type demoRecipe struct {
	key         string
	name        name
	ingredients []ingredient
	yield       float64
	portions    int
}

var recipes = []demoRecipe{
	{
		key: "soup", name: name{"Red lentil soup", "Juha od crvene leće"},
		ingredients: []ingredient{{"lentils", 300}, {"onion", 150}, {"tomatoes", 400}, {"oil", 20}},
		yield:       1800, portions: 6,
	},
	{
		key: "bolognese", name: name{"Bolognese sauce", "Bolonjez umak"},
		ingredients: []ingredient{{"beef", 500}, {"tomatoes", 800}, {"onion", 150}, {"oil", 15}},
		yield:       1200, portions: 5,
	},
}

// part is an item of a meal's template: grams of a food, scaled with the
// day's appetite when scale is set, or a number of its servings.
type part struct {
	food     string
	amount   float64
	servings bool
	scale    bool
}

// g is grams of a food that stay as they are; s grams that scale; n a
// number of a food's servings.
func g(f string, amount float64) part { return part{food: f, amount: amount} }
func s(f string, amount float64) part { return part{food: f, amount: amount, scale: true} }
func n(f string, count float64) part  { return part{food: f, amount: count, servings: true} }

// The meals of a day, each a choice of templates.
var (
	breakfasts = [][]part{
		{s("oats", 70), s("milk", 250), n("banana", 1), n("whey", 1)},
		{n("egg", 3), n("bread", 2), n("tomato", 1), g("cottage", 150)},
		{g("yoghurt", 300), g("blueberries", 100), s("oats", 40), n("whey", 1)},
	}
	lunches = [][]part{
		{g("chicken", 200), s("rice", 250), g("broccoli", 150), g("oil", 10)},
		{s("pasta", 220), n("bolognese", 1), g("cottage", 100)},
		{g("tuna", 150), n("wrap", 2), g("salad", 80), s("cheese", 20)},
		{n("soup", 1), s("bread", 80), g("chicken", 120)},
	}
	dinners = [][]part{
		{g("salmon", 180), s("potatoes", 300), g("salad", 100), g("oil", 10)},
		{g("chicken", 220), s("rice", 200), g("broccoli", 150)},
		{n("egg", 2), g("tuna", 120), s("potatoes", 250), g("salad", 80)},
		{s("pasta", 200), n("bolognese", 1), g("salad", 80)},
		{n("soup", 1), s("bread", 60), g("tuna", 150)},
	}
	snacks = [][]part{
		{n("apple", 1), s("almonds", 20)},
		{g("cottage", 200), s("blueberries", 80)},
		{n("whey", 1), n("banana", 1)},
		{g("yoghurt", 200), s("chocolate", 15)},
		{n("bread", 1), n("peanut", 1)},
		{g("yoghurt", 200), s("oats", 30)},
	}
)
