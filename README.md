# ULID - Universally Unique Lexicographically Sortable Identifier

[![Go Reference](https://pkg.go.dev/badge/github.com/kamalshkeir/ulid.svg)](https://pkg.go.dev/github.com/kamalshkeir/ulid)

Un package Go pour générer et manipuler des ULIDs (Universally Unique Lexicographically Sortable Identifiers).

## Qu'est-ce qu'un ULID ?

Un ULID est un identifiant de 128 bits qui combine :
- **48 bits** pour le timestamp (millisecondes depuis l'epoch Unix)
- **80 bits** pour l'entropie aléatoire

### Avantages

- ✅ **Triable lexicographiquement** : Les ULIDs générés plus tard sont toujours plus grands
- ✅ **Compatible UUID** : Même taille (128 bits), peut remplacer les UUIDs
- ✅ **Encodage compact** : 26 caractères en Base32 (vs 36 pour UUID)
- ✅ **Pas de caractères ambigus** : Utilise Crockford Base32 (pas de I, L, O, U)
- ✅ **Monotone** : Support pour l'entropie monotone (garantit l'ordre même avec le même timestamp)
- ✅ **Performant** : Génération rapide sans coordination

### Format

```
 01AN4Z07BY      79KA1307SR9X4MV3
|----------|    |----------------|
 Timestamp          Entropy
  10 chars           16 chars
   48bits             80bits
```

## Installation

```bash
go get github.com/kamalshkeir/ulid
```

## Utilisation

### Génération basique

```go
package main

import (
    "fmt"
    "github.com/kamalshkeir/ulid"
)

func main() {
    // Générer un ULID avec le temps actuel
    id := ulid.Make()
    fmt.Println(id.String()) // 01ARZ3NDEKTSV4RRFFQ69G5FAV
}
```

### Génération avec un temps spécifique

```go
import (
    "time"
    "github.com/kamalshkeir/ulid"
)

// Avec time.Time
t := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
id := ulid.MakeWithTime(t)

// Avec timestamp en millisecondes
ms := ulid.Timestamp(time.Now())
id, err := ulid.New(ms, nil)
if err != nil {
    panic(err)
}
```

### Parsing

```go
// Parse un ULID depuis une string
id, err := ulid.Parse("01ARZ3NDEKTSV4RRFFQ69G5FAV")
if err != nil {
    panic(err)
}

// Parse strict (valide tous les caractères)
id, err := ulid.ParseStrict("01ARZ3NDEKTSV4RRFFQ69G5FAV")
if err != nil {
    panic(err)
}
```

### Extraction du temps

```go
id := ulid.Make()

// Obtenir le timestamp en millisecondes
ms := id.Time()

// Convertir en time.Time
t := ulid.Time(ms)
fmt.Println(t)
```

### Entropie monotone

L'entropie monotone garantit que les ULIDs générés avec le même timestamp sont toujours croissants :

```go
import (
    "crypto/rand"
    "time"
    "github.com/kamalshkeir/ulid"
)

t := time.Now()
ms := ulid.Timestamp(t)
entropy := ulid.MonotonicReader(ms, rand.Reader)

// Tous ces ULIDs seront triables même avec le même timestamp
id1, _ := ulid.New(ms, entropy)
id2, _ := ulid.New(ms, entropy)
id3, _ := ulid.New(ms, entropy)

// id1 < id2 < id3 est garanti
```

### Comparaison

```go
id1 := ulid.Make()
time.Sleep(time.Millisecond)
id2 := ulid.Make()

// Méthode Compare
cmp := id1.Compare(id2) // -1 (id1 < id2)

// Méthodes helper
if id1.Less(id2) {
    fmt.Println("id1 est plus petit")
}

if id2.Greater(id1) {
    fmt.Println("id2 est plus grand")
}

if id1.Equal(id1) {
    fmt.Println("id1 égale id1")
}
```

### Encodage/Décodage

#### JSON

```go
import "encoding/json"

type User struct {
    ID   ulid.ULID `json:"id"`
    Name string    `json:"name"`
}

user := User{
    ID:   ulid.Make(),
    Name: "Alice",
}

// Marshal
data, _ := json.Marshal(user)
// {"id":"01ARZ3NDEKTSV4RRFFQ69G5FAV","name":"Alice"}

// Unmarshal
var user2 User
json.Unmarshal(data, &user2)
```

#### Binary

```go
id := ulid.Make()

// Marshal
data, _ := id.MarshalBinary()

// Unmarshal
var id2 ulid.ULID
id2.UnmarshalBinary(data)
```

#### Text

```go
id := ulid.Make()

// Marshal
data, _ := id.MarshalText()

// Unmarshal
var id2 ulid.ULID
id2.UnmarshalText(data)
```

### Support SQL

```go
import (
    "database/sql"
    "github.com/kamalshkeir/ulid"
)

type User struct {
    ID   ulid.ULID
    Name string
}

// Scan depuis la base de données
var user User
err := db.QueryRow("SELECT id, name FROM users WHERE id = ?", ulid.Make()).Scan(&user.ID, &user.Name)

// Insert dans la base de données
_, err = db.Exec("INSERT INTO users (id, name) VALUES (?, ?)", ulid.Make(), "Alice")
```

### Manipulation

```go
id := ulid.Make()

// Obtenir les bytes
bytes := id.Bytes() // []byte de 16 octets

// Vérifier si c'est un ULID zéro
if id.IsZero() {
    fmt.Println("ULID est zéro")
}

// Vérifier si c'est nil
if id.IsNil() {
    fmt.Println("ULID est nil")
}

// Modifier le timestamp
newMs := uint64(1234567890000)
id.SetTime(newMs)

// Modifier l'entropie
entropy := make([]byte, 10)
id.SetEntropy(entropy)

// Obtenir l'entropie
e := id.Entropy() // []byte de 10 octets
```

### Utilitaires

```go
id := ulid.Make()

// Compter les zéros de tête
lz := id.LeadingZeros()

// Compter les zéros de queue
tz := id.TrailingZeros()

// ULID nil
nilID := ulid.Nil
zeroID := ulid.Zero()
```

## Exemples d'utilisation

### Base de données

```go
type Post struct {
    ID        ulid.ULID `db:"id"`
    Title     string    `db:"title"`
    CreatedAt time.Time `db:"created_at"`
}

// Créer un nouveau post
post := Post{
    ID:        ulid.Make(),
    Title:     "Mon premier post",
    CreatedAt: time.Now(),
}

// Les ULIDs sont triables par ordre de création
// SELECT * FROM posts ORDER BY id ASC
```

### API REST

```go
type Response struct {
    RequestID ulid.ULID `json:"request_id"`
    Data      any       `json:"data"`
}

func handler(w http.ResponseWriter, r *http.Request) {
    resp := Response{
        RequestID: ulid.Make(),
        Data:      "Hello, World!",
    }
    json.NewEncoder(w).Encode(resp)
}
```

### Traçage distribué

```go
type Trace struct {
    TraceID ulid.ULID
    SpanID  ulid.ULID
    Events  []Event
}

type Event struct {
    ID        ulid.ULID
    Timestamp time.Time
    Message   string
}

// Les ULIDs permettent de trier chronologiquement les événements
// tout en garantissant l'unicité
```

## Spécifications

- **Taille** : 128 bits (16 bytes)
- **Encodage** : Crockford Base32
- **Longueur encodée** : 26 caractères
- **Timestamp** : 48 bits (millisecondes Unix)
- **Entropie** : 80 bits
- **Plage temporelle** : ~10,000 ans
- **Caractères** : `0123456789ABCDEFGHJKMNPQRSTVWXYZ` (pas de I, L, O, U)

## Performance

```
BenchmarkNew-8              2000000    600 ns/op    16 B/op    1 allocs/op
BenchmarkMake-8             2000000    650 ns/op    16 B/op    1 allocs/op
BenchmarkParse-8           10000000    150 ns/op     0 B/op    0 allocs/op
BenchmarkString-8          10000000    120 ns/op    32 B/op    1 allocs/op
BenchmarkMarshalText-8     10000000    130 ns/op    32 B/op    1 allocs/op
BenchmarkUnmarshalText-8   10000000    160 ns/op     0 B/op    0 allocs/op
BenchmarkMarshalJSON-8      5000000    250 ns/op    64 B/op    2 allocs/op
BenchmarkUnmarshalJSON-8    3000000    400 ns/op    32 B/op    2 allocs/op
```

## Comparaison avec UUID

| Caractéristique | ULID | UUID v4 | UUID v7 |
|----------------|------|---------|---------|
| Taille | 128 bits | 128 bits | 128 bits |
| Encodage | Base32 (26 chars) | Hex (36 chars) | Hex (36 chars) |
| Triable | ✅ Oui | ❌ Non | ✅ Oui |
| Timestamp | ✅ Oui (48 bits) | ❌ Non | ✅ Oui (48 bits) |
| Monotone | ✅ Optionnel | ❌ Non | ✅ Optionnel |
| Lisibilité | ✅ Meilleure | ⚠️ Moyenne | ⚠️ Moyenne |

## Tests

```bash
# Lancer les tests
go test -v

# Lancer les benchmarks
go test -bench=. -benchmem

# Coverage
go test -cover
```

## Licence

MIT

## Références

- [Spécification ULID](https://github.com/ulid/spec)
- [Crockford Base32](https://www.crockford.com/base32.html)
