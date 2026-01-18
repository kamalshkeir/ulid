package main

import (
	"fmt"
	"time"

	"github.com/kamalshkeir/ulid"
)

func main() {
	// Exemple 1: Génération basique
	fmt.Println("=== Génération basique ===")
	id1 := ulid.Make()
	fmt.Printf("ULID: %s\n", id1.String())
	fmt.Printf("Timestamp: %d\n", id1.Time())
	fmt.Printf("Time: %s\n\n", ulid.Time(id1.Time()))

	// Exemple 2: Génération avec un temps spécifique
	fmt.Println("=== Génération avec temps spécifique ===")
	specificTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	id2 := ulid.MakeWithTime(specificTime)
	fmt.Printf("ULID: %s\n", id2.String())
	fmt.Printf("Time: %s\n\n", ulid.Time(id2.Time()))

	// Exemple 3: Parsing
	fmt.Println("=== Parsing ===")
	parsed, err := ulid.Parse("01ARZ3NDEKTSV4RRFFQ69G5FAV")
	if err != nil {
		panic(err)
	}
	fmt.Printf("Parsed ULID: %s\n", parsed.String())
	fmt.Printf("Time: %s\n\n", ulid.Time(parsed.Time()))

	// Exemple 4: Comparaison
	fmt.Println("=== Comparaison ===")
	id3 := ulid.Make()
	time.Sleep(2 * time.Millisecond)
	id4 := ulid.Make()

	fmt.Printf("ID3: %s\n", id3.String())
	fmt.Printf("ID4: %s\n", id4.String())
	fmt.Printf("ID3 < ID4: %v\n", id3.Less(id4))
	fmt.Printf("ID4 > ID3: %v\n\n", id4.Greater(id3))

	// Exemple 5: Sortabilité
	fmt.Println("=== Sortabilité ===")
	ids := make([]ulid.ULID, 5)
	for i := 0; i < 5; i++ {
		ids[i] = ulid.Make()
		time.Sleep(time.Millisecond)
	}

	fmt.Println("ULIDs générés dans l'ordre:")
	for i, id := range ids {
		fmt.Printf("%d: %s (time: %s)\n", i+1, id.String(), ulid.Time(id.Time()).Format("15:04:05.000"))
	}

	// Vérifier qu'ils sont triés
	sorted := true
	for i := 1; i < len(ids); i++ {
		if !ids[i-1].Less(ids[i]) {
			sorted = false
			break
		}
	}
	fmt.Printf("\nTriés correctement: %v\n\n", sorted)

	// Exemple 6: Entropie monotone
	fmt.Println("=== Entropie monotone ===")
	ms := ulid.Timestamp(time.Now())
	entropy := ulid.MonotonicReader(ms, nil)

	fmt.Println("ULIDs avec le même timestamp mais entropie monotone:")
	for i := 0; i < 3; i++ {
		id, _ := ulid.New(ms, entropy)
		fmt.Printf("%d: %s\n", i+1, id.String())
	}
	fmt.Println()

	// Exemple 7: Manipulation
	fmt.Println("=== Manipulation ===")
	id5 := ulid.Make()
	fmt.Printf("ULID original: %s\n", id5.String())

	// Modifier le timestamp
	newMs := uint64(1704067200000) // 2024-01-01 00:00:00 UTC
	id5.SetTime(newMs)
	fmt.Printf("Après SetTime: %s\n", id5.String())
	fmt.Printf("Nouveau temps: %s\n\n", ulid.Time(id5.Time()))

	// Exemple 8: Vérifications
	fmt.Println("=== Vérifications ===")
	var zero ulid.ULID
	fmt.Printf("ULID zéro: %s\n", zero.String())
	fmt.Printf("IsZero: %v\n", zero.IsZero())
	fmt.Printf("IsNil: %v\n", zero.IsNil())

	id6 := ulid.Make()
	fmt.Printf("\nULID normal: %s\n", id6.String())
	fmt.Printf("IsZero: %v\n", id6.IsZero())
	fmt.Printf("IsNil: %v\n", id6.IsNil())
}
