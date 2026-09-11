package usecase

import "testing"

func TestQuestCatalog(t *testing.T) {
	if required, ok := QuestRequirement("starter_collect", "collect_coin"); !ok || required != 2 {
		t.Fatalf("starter_collect required=%d ok=%v", required, ok)
	}
	if _, ok := QuestRequirement("missing", "collect_coin"); ok {
		t.Fatal("unknown quest must not resolve")
	}
	if _, ok := QuestRequirement("starter_collect", "missing"); ok {
		t.Fatal("unknown objective must not resolve")
	}

	reward := QuestReward("starter_collect")
	if reward["gem"] != 1 {
		t.Fatalf("starter_collect reward=%v", reward)
	}
	reward["gem"] = 999
	if QuestReward("starter_collect")["gem"] != 1 {
		t.Fatal("QuestReward must return a copy")
	}
	if QuestReward("missing") != nil {
		t.Fatal("unknown quest reward must be nil")
	}

	ids := QuestIDs()
	if len(ids) < 3 {
		t.Fatalf("expected at least 3 catalog quests, got %v", ids)
	}
	for i := 1; i < len(ids); i++ {
		if ids[i-1] >= ids[i] {
			t.Fatalf("QuestIDs not sorted: %v", ids)
		}
	}
	for _, id := range ids {
		def := questCatalog[id]
		if len(def.Objectives) == 0 || len(def.Reward) == 0 {
			t.Fatalf("quest %s is incomplete: %+v", id, def)
		}
		for objective, required := range def.Objectives {
			if required <= 0 {
				t.Fatalf("quest %s objective %s required=%d", id, objective, required)
			}
		}
		for item, amount := range def.Reward {
			if amount <= 0 {
				t.Fatalf("quest %s reward %s=%d", id, item, amount)
			}
		}
	}
}
