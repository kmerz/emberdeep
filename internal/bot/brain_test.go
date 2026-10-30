package bot

import (
	"testing"

	"emberdeep/internal/proto"
)

func open(x, y int) bool { return x >= 0 && y >= 0 && x < 60 && y < 30 }

func view(cls string, ents ...Ent) *View {
	return &View{Me: 1, Cls: cls, Ents: ents, Walkable: open, Unknown: func(x, y int) bool { return x >= 60 },
		CD: []int{0, 0, 0}, Cost: []int{0, 10, 10}, Res: 100, HP: 100, HPMax: 100, Potions: 2}
}

func find(msgs []proto.ClientMsg, t string) *proto.ClientMsg {
	for i := range msgs {
		if msgs[i].T == t {
			return &msgs[i]
		}
	}
	return nil
}

func TestFollowsNearestHuman(t *testing.T) {
	b := NewBrain(false)
	msgs := b.Think(view("rogue", Ent{ID: 1, K: "h", X: 10, Y: 10}, Ent{ID: 2, K: "h", X: 30, Y: 10}))
	m := find(msgs, proto.CMove)
	if m == nil || dirs[m.Dir][0] != 1 {
		t.Fatalf("expected a step east toward the human, got %+v", msgs)
	}
	if msgs = b.Think(view("rogue", Ent{ID: 1, K: "h", X: 10, Y: 10}, Ent{ID: 2, K: "h", X: 12, Y: 10})); len(msgs) != 0 {
		t.Fatal("stay put when close to the human")
	}
}

func TestAttacksNearbyEnemiesWithSkills(t *testing.T) {
	b := NewBrain(false)
	msgs := b.Think(view("warrior", Ent{ID: 1, K: "h", X: 10, Y: 10}, Ent{ID: 10001, K: "m", X: 11, Y: 10}))
	if c := find(msgs, proto.CCast); c == nil || c.Slot != 1 {
		t.Fatalf("warrior should raise Iron Wall first, got %+v", msgs)
	}
	v := view("warrior", Ent{ID: 1, K: "h", X: 10, Y: 10}, Ent{ID: 10001, K: "m", X: 11, Y: 10})
	v.CD = []int{0, 50, 50}
	if c := find(b.Think(v), proto.CCast); c == nil || c.Slot != 0 {
		t.Fatal("then Cleave when others are on cooldown")
	}
	v = view("sorcerer", Ent{ID: 1, K: "h", X: 10, Y: 10}, Ent{ID: 10001, K: "m", X: 16, Y: 10})
	if c := find(b.Think(v), proto.CCast); c == nil || c.Slot != 1 {
		t.Fatal("sorcerer opens with Ember Nova off cooldown")
	}
	v.Res = 0
	v.Cost = []int{5, 40, 20}
	if c := find(b.Think(v), proto.CCast); c != nil {
		t.Fatal("no cast without mana")
	}
	v = view("rogue", Ent{ID: 1, K: "h", X: 10, Y: 10}, Ent{ID: 10001, K: "m", X: 14, Y: 10})
	if find(b.Think(v), proto.CMove) == nil && find(b.Think(v), proto.CCast) == nil {
		t.Fatal("rogue should act against an enemy 4 cells away")
	}
}

func TestRevivesDownedAllyAndUsesPotion(t *testing.T) {
	b := NewBrain(false)
	msgs := b.Think(view("monk", Ent{ID: 1, K: "h", X: 10, Y: 10}, Ent{ID: 2, K: "h", X: 11, Y: 10, Downed: true}))
	if r := find(msgs, proto.CRevive); r == nil || !r.Hold || r.Target != 2 {
		t.Fatalf("expected revive hold, got %+v", msgs)
	}
	if len(b.Think(view("monk", Ent{ID: 1, K: "h", X: 10, Y: 10}, Ent{ID: 2, K: "h", X: 11, Y: 10, Downed: true}))) != 0 {
		t.Fatal("keep holding without spamming")
	}
	msgs = b.Think(view("monk", Ent{ID: 1, K: "h", X: 10, Y: 10}, Ent{ID: 2, K: "h", X: 20, Y: 10, Downed: true}))
	if find(msgs, proto.CMove) == nil {
		t.Fatal("walk to a downed ally")
	}
	v := view("monk", Ent{ID: 1, K: "h", X: 10, Y: 10}, Ent{ID: 10001, K: "m", X: 11, Y: 10})
	v.HP = 20
	if find(b.Think(v), proto.CPotion) == nil {
		t.Fatal("drink a potion when low")
	}
	if len(b.Think(view("monk", Ent{ID: 1, K: "h", X: 10, Y: 10, Downed: true}))) != 0 {
		t.Fatal("downed bots do nothing")
	}
}

func TestLeaderExplores(t *testing.T) {
	b := NewBrain(true)
	msgs := b.Think(view("monk", Ent{ID: 1, K: "h", X: 10, Y: 10}, Ent{ID: 2, K: "h", X: 11, Y: 10}))
	m := find(msgs, proto.CMove)
	if m == nil || dirs[m.Dir][0] != 1 {
		t.Fatalf("leader should head toward unexplored space (east), got %+v", msgs)
	}
}
