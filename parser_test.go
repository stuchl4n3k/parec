package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseReceipt_Samples(t *testing.T) {
	cases := []struct {
		name    string
		file    string
		want    []Item
		wantSum int64
	}{
		{
			name: "sample00",
			file: "sample00.txt",
			want: []Item{
				{Name: "Antonínovo pekařství Karlínský rohlík 50g", PriceDecimals: 3450},
				{Name: "Paranit Radikální šampon+hřeben 1ks", PriceDecimals: 33990},
			},
			wantSum: 37440,
		},
		{
			name: "sample01",
			file: "sample01.txt",
			want: []Item{
				{Name: "Barilla Pesto Rosso 200g", PriceDecimals: 6790},
				{Name: "Harmony Prima Bob a Bobek papírové kapesníky 3vrstvé, 10×10 ks 10ks", PriceDecimals: 3490},
				{Name: "Hugo Žvýkačky bez aspartamu Fresh Fruit 9g", PriceDecimals: 2490},
				{Name: "Hugo Žvýkačky bez aspartamu Skořice 9g", PriceDecimals: 2490},
				{Name: "Hugo Žvýkačky bez aspartamu spearmint 9g", PriceDecimals: 2490},
				{Name: "Jojo Kyselé žížalky želé bonbóny s ovocnými příchutěmi 80g", PriceDecimals: 4180},
				{Name: "Miléne Mléko a med tekuté mýdlo náhradní náplň 500ml", PriceDecimals: 4290},
				{Name: "Moddia Kosmetické ubrousky 2vrstvé box 200ks", PriceDecimals: 4842},
				{Name: "Moravia Podmáslí kysané 1 % 500ml", PriceDecimals: 2090},
				{Name: "Rohlik.cz Mozzarella Fiordilate 200g", PriceDecimals: 5941},
				{Name: "Sláma Mandlová paštika 200g", PriceDecimals: 6090},
				{Name: "Výběrová šunka", PriceDecimals: 3120},
				{Name: "Zott Protein mozzarella classic 125g", PriceDecimals: 3890},
				{Name: "MOVit Zinek Chelát 15 mg tbl.90 90ks", PriceDecimals: 12990},
			},
			wantSum: 65183,
		},
		{
			name: "sample02",
			file: "sample02.txt",
			want: []Item{
				{Name: "Alnatura BIO Citronová šťáva sklo 200ml", PriceDecimals: 4131},
				{Name: "Aroy-D BIO kokosové mléko 250ml", PriceDecimals: 4290},
				{Name: "Banán 1 ks", PriceDecimals: 3711},
				{Name: "BioSaurus BIO Kukuřičný snack sýrový 50g", PriceDecimals: 3321},
				{Name: "Borůvky, kyblík 350g", PriceDecimals: 9990},
				{Name: "Cereabar BIO Flapjack peanut & chocolate 60g", PriceDecimals: 7002},
				{Name: "Fala Pekařské droždí čerstvé 42g", PriceDecimals: 1290},
				{Name: "Farmářská brambora pozdní nepraná, volně", PriceDecimals: 3034},
				{Name: "Hrozny tmavé bezsemenné, balení 500g", PriceDecimals: 7990},
				{Name: "Cherry rajčata (rodinné balení), balení 500g", PriceDecimals: 6990},
				{Name: "Jablko červené malé 1 ks", PriceDecimals: 1915},
				{Name: "Jahody čerstvé, vanička 250g", PriceDecimals: 9990},
				{Name: "Kitchin Spaghetti N. 5 500g", PriceDecimals: 4122},
				{Name: "Kiwi RTE (cca 80 g) 1ks", PriceDecimals: 4580},
				{Name: "Limeta 1 ks cca 1ks", PriceDecimals: 2580},
				{Name: "Madeta Sýrařův výběr Moravský bochník 45 % plátky 100g", PriceDecimals: 3690},
				{Name: "Maliny čerstvé, vanička 125g", PriceDecimals: 6090},
				{Name: "Mandarinky, balení", PriceDecimals: 4915},
				{Name: "Miil BIO Čerstvé mléko plnotučné 3,6% 1l", PriceDecimals: 3141},
				{Name: "Miil Camembert 120g", PriceDecimals: 2691},
				{Name: "Miil Čerstvá smetana ke šlehání 33% 250ml", PriceDecimals: 3501},
				{Name: "Miil Extra tvrdý sýr italského typu strouhaný 32% 100g", PriceDecimals: 4592},
				{Name: "Miil Mozzarella 42% 125g", PriceDecimals: 2241},
				{Name: "Mlékárna Kunín Athentikos jogurt bílý 400g", PriceDecimals: 3090},
				{Name: "Moddia Toaletní papír 3vrstvý, 10 ks 286m", PriceDecimals: 9891},
				{Name: "Natu BIO Spirulina prášek 80g", PriceDecimals: 11691},
				{Name: "Natura Perlivá voda (6×1,5l) 9l", PriceDecimals: 8990},
				{Name: "Okurka hadovka (cca 300 g) 1ks", PriceDecimals: 2790},
				{Name: "Pappudia Jablečný džus 100% 1l", PriceDecimals: 4041},
				{Name: "Pappudia Pomerančový džus 100% RFA 1l", PriceDecimals: 6291},
				{Name: "Pernerka Pšeničná hladká mouka 1kg", PriceDecimals: 3190},
				{Name: "Relax 100% ananas 1l", PriceDecimals: 15580},
				{Name: "Rohlíkův sádlový rohlík 50g", PriceDecimals: 3355},
				{Name: "Rohlik.cz Gouda holandská 48+", PriceDecimals: 5318},
				{Name: "Rohlik.cz Párek pro děti bez přidaného dusitanu", PriceDecimals: 8446},
				{Name: "Rohlik.cz Párek pro děti bez přidaného dusitanu", PriceDecimals: 8506},
				{Name: "Spak Master Rajčatový protlak 180g", PriceDecimals: 5290},
				{Name: "Výběrová šunka", PriceDecimals: 3120},
				{Name: "Well Well BIO tofu tvrdé v kelímku s vodou 400g", PriceDecimals: 4690},
				{Name: "5 Stagioni hladká mouka typu 00 na neapolskou pizzu 1kg", PriceDecimals: 6890},
			},
			wantSum: 216966,
		},
		{
			name: "sample03",
			file: "sample03.txt",
			want: []Item{
				{Name: "Alnatura BIO Rajčata sekaná 240g", PriceDecimals: 5571},
				{Name: "Amora Dijonská hořčice ostrá 430g", PriceDecimals: 12490},
				{Name: "Ananas Sweet gold (cca 1,1 kg) 1ks", PriceDecimals: 6990},
				{Name: "Balený kostkový led 1kg", PriceDecimals: 7180},
				{Name: "Banán 1 ks", PriceDecimals: 4247},
				{Name: "BioSaurus BIO Kukuřičný snack sýrový 50g", PriceDecimals: 2190},
				{Name: "Birell Polotmavý 6×0,5 l plech 3l", PriceDecimals: 15990},
				{Name: "Birell Pomelo & grep nealkoholický 6×0,5 l plech 3l", PriceDecimals: 15990},
				{Name: "Borůvky, kyblík 350g", PriceDecimals: 13990},
				{Name: "Carchelejo Fuet Imperial Extra salám 160g", PriceDecimals: 6990},
				{Name: "Celer řapíkatý 1 ks 350g", PriceDecimals: 3790},
				{Name: "Cirkulka Kofola Original sklo 1l", PriceDecimals: 3690},
				{Name: "Čerstvě utrženo - Rajčata cherry na větvičce odr. Strabena, střapec", PriceDecimals: 5173},
				{Name: "Dobroty s příběhem Nakládaný hermelín s cibulí", PriceDecimals: 42490},
				{Name: "Domol Žlučové mýdlo 100g", PriceDecimals: 2490},
				{Name: "Dr. Antonio Martins BIO Kokosové mléko 3,8% 1l", PriceDecimals: 7590},
				{Name: "Farmářská Vejce ze dvora z volného chovu 10ks", PriceDecimals: 10990},
				{Name: "Filippo Berio Pesto ze sušených rajčat 190g", PriceDecimals: 7790},
				{Name: "Franz Josef Kaiser Rajčata sušená krájená v oleji 110g", PriceDecimals: 5590},
				{Name: "Guau Guacamole 200g", PriceDecimals: 6490},
				{Name: "Hamé Ovocná směs linecká 260g", PriceDecimals: 4690},
				{Name: "Jablko červené malé 1 ks", PriceDecimals: 2454},
				{Name: "Kotányi Skořice celá 17g", PriceDecimals: 2190},
				{Name: "Lambertz Perníčky s cukrovou polevou 160g", PriceDecimals: 2590},
				{Name: "Leerdammer Original srdce 440g", PriceDecimals: 13990},
				{Name: "Lef Linecké těsto vanilkové 400g", PriceDecimals: 4290},
				{Name: "Lilek 1 ks", PriceDecimals: 6194},
				{Name: "Linteo Premium papírové kapesníky 3vrstvé box 60ks", PriceDecimals: 4290},
				{Name: "Madeta Blaťácké zlato s vlašskými ořechy 120g", PriceDecimals: 4040},
				{Name: "Maliny čerstvé, vanička 125g", PriceDecimals: 6090},
				{Name: "Maso Klouda Trhané hovězí maso", PriceDecimals: 17917},
				{Name: "Máta řezaná, balení 30g", PriceDecimals: 2890},
				{Name: "Merhautovo pekařství Těsto ořechové světlé 500g", PriceDecimals: 11990},
				{Name: "Merhautovo pekařství Těsto perníkové 500g", PriceDecimals: 11990},
				{Name: "Miil BIO Čerstvé mléko plnotučné 3,6% 1l", PriceDecimals: 3141},
				{Name: "Miil Camembert 120g", PriceDecimals: 5382},
				{Name: "Miil Gouda 47% bloček 250g", PriceDecimals: 4491},
				{Name: "Miil Mozzarella 42% 125g", PriceDecimals: 6723},
				{Name: "Moddia Papír na pečení archy, 38×42 cm 30ks", PriceDecimals: 2691},
				{Name: "Mrkev s natí, svazek (cca 500 g) 1ks", PriceDecimals: 3590},
				{Name: "Natura Perlivá voda (6×1,5l) 9l", PriceDecimals: 8990},
				{Name: "Natural Jihlava Tahini 200g", PriceDecimals: 10490},
				{Name: "Nuevo Progreso Tortilla chips restaurant style 400g", PriceDecimals: 19980},
				{Name: "Okurka hadovka (cca 300 g) 1ks", PriceDecimals: 6180},
				{Name: "Olymp Kalamata olivy bez pecky 150g", PriceDecimals: 8790},
				{Name: "Olymp Zelené olivy s papričkou 320g", PriceDecimals: 7490},
				{Name: "Pappudia Jablečný džus 100% 1l", PriceDecimals: 4041},
				{Name: "Pappudia Pomerančový džus 100% RFA 1l", PriceDecimals: 12582},
				{Name: "Pekárna Kabát Celozrnná houska 50g", PriceDecimals: 3364},
				{Name: "Pfanner Borůvka 1l", PriceDecimals: 7090},
				{Name: "Pomelo červené 1 ks cca 900g 1ks", PriceDecimals: 2990},
				{Name: "Pomeranč 1 ks", PriceDecimals: 1597},
				{Name: "Relax 100% ananas 1l", PriceDecimals: 7790},
				{Name: "Rohlíkova Bageta francouzská klasik 240g", PriceDecimals: 3391},
				{Name: "Rohlíkova Bageta francouzská rustikální 280g", PriceDecimals: 3901},
				{Name: "Rohlik.cz Ginger Shot se spirulinou 130ml", PriceDecimals: 5601},
				{Name: "Rohlik.cz Obložená mísa šunka, salám 500g", PriceDecimals: 21241},
				{Name: "Rossini Primitivo Puglia IGT 0.75l", PriceDecimals: 9990},
				{Name: "Rozmarýn řezaný, balení 30g", PriceDecimals: 2590},
				{Name: "Rukola, vanička 125g", PriceDecimals: 4390},
				{Name: "Ředkvičky bez natě, balení 300g", PriceDecimals: 1890},
				{Name: "Směs listových salátů 250g", PriceDecimals: 4850},
				{Name: "Top Topic Original limonáda 1.5l", PriceDecimals: 3290},
				{Name: "Vital Snack BIO Čočkové chipsy s mořskou solí 65g", PriceDecimals: 2691},
				{Name: "Well Well BIO tofu tvrdé v kelímku s vodou 400g", PriceDecimals: 4690},
				{Name: "Yutto Kešu ořechy jádra pražená, solená 150g", PriceDecimals: 5391},
				{Name: "Zelenkova Krůtí klobása", PriceDecimals: 8997},
				{Name: "Žitný rohlík 60g", PriceDecimals: 2670},
			},
			wantSum: 496281,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text := readFixture(t, tc.file)
			got, err := ParseReceipt(strings.NewReader(text))
			if err != nil {
				t.Fatalf("ParseReceipt() error = %v", err)
			}
			assertItemsEqual(t, got, tc.want)

			gotSum := sumItems(got)
			if gotSum != tc.wantSum {
				t.Errorf("sumItems() = %v, want %v", gotSum, tc.wantSum)
			}
		})
	}
}

func TestParseReceipt_NoItemsSection_ReturnsEmptyList(t *testing.T) {
	const input = `
DODACÍ LIST
Objednávka #123
Nějaký text
Cena celkem 123,45 Kč
`
	got, err := ParseReceipt(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) > 0 {
		t.Fatalf("unexpected items: %v", got)
	}
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("data", name)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error: %v", path, err)
	}
	return string(b)
}

func assertItemsEqual(t *testing.T, got []*Item, want []Item) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("item count mismatch: got=%d want=%d", len(got), len(want))
	}

	for i := range want {
		if got[i].Name != want[i].Name || got[i].PriceDecimals != want[i].PriceDecimals {
			t.Fatalf("item mismatch at index %d:\n  got:  %+v\n  want: %+v", i, *got[i], want[i])
		}
	}
}

func sumItems(items []*Item) int64 {
	var sum int64
	for _, it := range items {
		sum += it.PriceDecimals
	}
	return sum
}
