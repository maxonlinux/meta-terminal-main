package types

const (
	BUCKET_AVAILABLE int8 = 0
	BUCKET_LOCKED    int8 = 1
	BUCKET_MARGIN    int8 = 2
)

type UserBalance struct {
	UserID  UserID
	Asset   string
	Buckets map[int8]int64 // 0=Available, 1=Locked, 2=Margin
	Version int64
}

func NewUserBalance(userID UserID, asset string) *UserBalance {
	return &UserBalance{
		UserID:  userID,
		Asset:   asset,
		Buckets: make(map[int8]int64),
	}
}

func (b *UserBalance) Get(bucket int8) int64 {
	return b.Buckets[bucket]
}

func (b *UserBalance) Add(bucket int8, amount int64) {
	b.Buckets[bucket] += amount
}

func (b *UserBalance) Deduct(bucket int8, amount int64) bool {
	if b.Buckets[bucket] < amount {
		return false
	}
	b.Buckets[bucket] -= amount
	return true
}
