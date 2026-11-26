package world

// Lógica de F4: inventario, equipamiento, drops, tienda y mejora +N.

import (
	"math"
	"time"

	"munext/server/internal/combat"
	"munext/server/internal/gamedata"
	"munext/server/internal/items"
	"munext/server/internal/model"
	"munext/server/internal/protocol"
)

// Pickup recoge un drop del suelo.
type Pickup struct {
	SessionID int64
	DropID    int64
}

func (c Pickup) apply(w *World) { w.pickup(c) }

// Equip equipa un ítem del inventario.
type Equip struct {
	SessionID int64
	ItemID    int64
}

func (c Equip) apply(w *World) { w.equipItem(c) }

// Unequip desequipa un slot.
type Unequip struct {
	SessionID int64
	Slot      string
}

func (c Unequip) apply(w *World) { w.unequipItem(c) }

// InvMove mueve un ítem dentro de la grilla.
type InvMove struct {
	SessionID int64
	ItemID    int64
	X, Y      int
}

func (c InvMove) apply(w *World) { w.moveItem(c) }

// UseItem usa una poción del inventario.
type UseItem struct {
	SessionID int64
	ItemID    int64
}

func (c UseItem) apply(w *World) { w.useItem(c.SessionID, c.ItemID) }

// Enhance intenta mejorar un ítem.
type Enhance struct {
	SessionID int64
	ItemID    int64
}

func (c Enhance) apply(w *World) { w.enhanceItem(c) }

// ShopBuy compra un consumible del vendedor.
type ShopBuy struct {
	SessionID int64
	Tmpl      string
	Qty       int
}

func (c ShopBuy) apply(w *World) { w.shopBuy(c) }

// ShopSell vende un ítem del inventario.
type ShopSell struct {
	SessionID int64
	ItemID    int64
}

func (c ShopSell) apply(w *World) { w.shopSell(c) }

type drop struct {
	ID     int64
	MapID  string
	Tmpl   string
	Rarity string
	X, Z   float64
	Expire uint64
}

func (d *drop) entity(templates map[string]gamedata.ItemTemplate) protocol.Entity {
	name := d.Tmpl
	if t, ok := templates[d.Tmpl]; ok {
		name = t.Name
	}
	return protocol.Entity{
		ID: d.ID, Kind: "drop", Tmpl: d.Tmpl, Name: name, Rarity: d.Rarity,
		X: d.X, Y: 0, Z: d.Z,
	}
}

// --- Estadísticas derivadas del equipo ---

func (w *World) weaponProfile(p *player) (gamedata.ItemTemplate, model.Item, bool) {
	for _, it := range p.Items {
		if it.EquippedSlot != "weapon" {
			continue
		}
		t, ok := w.items[it.Tmpl]
		if !ok || t.Kind != "weapon" {
			return gamedata.ItemTemplate{}, model.Item{}, false
		}
		return t, it, true
	}
	return gamedata.ItemTemplate{}, model.Item{}, false
}

func (w *World) attackRange(p *player) float64 {
	if t, _, ok := w.weaponProfile(p); ok && t.Range > 0 {
		return t.Range
	}
	return combat.AttackRange(p.Class)
}

func (w *World) damageRange(p *player) (int, int) {
	profile := combat.Profile(p.Class)
	baseMin, baseMax := profile.MinDamage, profile.MaxDamage
	if t, it, ok := w.weaponProfile(p); ok {
		baseMin, baseMax = items.EffectiveDamage(t, it.Rarity, it.Enhance)
	}
	return combat.DamageRangeFromBase(p.stats(), p.Class, baseMin, baseMax)
}

func (w *World) attackInterval(p *player) time.Duration {
	baseMS := combat.Profile(p.Class).IntervalMS
	if t, _, ok := w.weaponProfile(p); ok && t.IntervalMS > 0 {
		baseMS = t.IntervalMS
	}
	return combat.AttackIntervalFromBase(p.stats(), baseMS)
}

func (w *World) armorDefense(p *player) int {
	armor := 0
	for _, it := range p.Items {
		if it.EquippedSlot == "" {
			continue
		}
		t, ok := w.items[it.Tmpl]
		if !ok || t.Slot == "weapon" || t.Slot == "" {
			continue
		}
		armor += items.EffectiveDefense(t, it.Rarity, it.Enhance)
	}
	return armor
}

func (w *World) totalDefense(p *player) int {
	return combat.Defense(p.stats()) + w.armorDefense(p)
}

func (w *World) potionCounts(p *player) (hp, mp int) {
	for _, it := range p.Items {
		if it.EquippedSlot != "" {
			continue
		}
		t, ok := w.items[it.Tmpl]
		if !ok || t.Kind != "potion" {
			continue
		}
		switch t.HealKind {
		case "hp":
			hp += it.Qty
		case "mp":
			mp += it.Qty
		}
	}
	return hp, mp
}

// --- Inventario ---

func (w *World) inventoryState(p *player) protocol.Inventory {
	list := make([]model.Item, len(p.Items))
	copy(list, p.Items)
	return protocol.Inventory{Gold: p.Gold, Items: list}
}

func (w *World) sendInventory(p *player) {
	data, err := protocol.Encode("inventory", w.inventoryState(p))
	if err != nil {
		w.log.Error("no se pudo codificar el inventario", "err", err)
		return
	}
	w.send(p, data)
}

func (w *World) afterInventoryChange(p *player) {
	w.sendInventory(p)
	w.sendSelfState(p)
	w.queueSave(p)
}

func (w *World) itemError(p *player, code, msg string) {
	w.sendError(p.Out, code, msg)
}

func findItemIndex(p *player, itemID int64) int {
	for i := range p.Items {
		if p.Items[i].ID == itemID {
			return i
		}
	}
	return -1
}

func (w *World) freeSlot(p *player) (int, int, bool) {
	var used [inventoryWidth][inventoryHeight]bool
	for _, it := range p.Items {
		if it.EquippedSlot != "" {
			continue
		}
		if it.InvX >= 0 && it.InvX < inventoryWidth && it.InvY >= 0 && it.InvY < inventoryHeight {
			used[it.InvX][it.InvY] = true
		}
	}
	for y := 0; y < inventoryHeight; y++ {
		for x := 0; x < inventoryWidth; x++ {
			if !used[x][y] {
				return x, y, true
			}
		}
	}
	return 0, 0, false
}

func (w *World) itemAt(p *player, x, y int) int {
	for i := range p.Items {
		it := &p.Items[i]
		if it.EquippedSlot == "" && it.InvX == x && it.InvY == y {
			return i
		}
	}
	return -1
}

func (w *World) removeItemAt(p *player, idx int) {
	p.Items = append(p.Items[:idx], p.Items[idx+1:]...)
}

// addItem agrega una instancia (o stack) al inventario. Devuelve false si no
// hay espacio; el caller debe restaurar el inventario si es atómico.
func (w *World) addItem(p *player, item model.Item) bool {
	tmpl, ok := w.items[item.Tmpl]
	if !ok {
		return false
	}
	if item.Qty < 1 {
		item.Qty = 1
	}
	if item.Rarity == "" {
		item.Rarity = items.RarityCommon
	}

	if tmpl.Stackable {
		remaining := item.Qty
		maxStack := max(1, tmpl.MaxStack)

		for i := range p.Items {
			if p.Items[i].Tmpl != item.Tmpl || p.Items[i].EquippedSlot != "" {
				continue
			}
			space := maxStack - p.Items[i].Qty
			if space <= 0 {
				continue
			}
			add := min(space, remaining)
			p.Items[i].Qty += add
			remaining -= add
			if remaining == 0 {
				return true
			}
		}

		first := true
		for remaining > 0 {
			x, y, ok := w.freeSlot(p)
			if !ok {
				return false
			}
			qty := min(maxStack, remaining)
			id := item.ID
			if !first || id == 0 {
				id = w.newItemID()
			}
			first = false
			p.Items = append(p.Items, model.Item{
				ID: id, CharacterID: p.ID, Tmpl: item.Tmpl, Enhance: item.Enhance,
				Rarity: item.Rarity, InvX: x, InvY: y, Qty: qty,
			})
			remaining -= qty
		}
		return true
	}

	x, y, ok := w.freeSlot(p)
	if !ok {
		return false
	}
	id := item.ID
	if id == 0 {
		id = w.newItemID()
	}
	p.Items = append(p.Items, model.Item{
		ID: id, CharacterID: p.ID, Tmpl: item.Tmpl, Enhance: item.Enhance,
		Rarity: item.Rarity, InvX: x, InvY: y, Qty: 1,
	})
	return true
}

func (w *World) newItemID() int64 {
	for {
		id := int64(w.rng.Uint64() & uint64(math.MaxInt64))
		if id == 0 {
			continue
		}
		if !w.itemIDInUse(id) {
			return id
		}
	}
}

func (w *World) itemIDInUse(id int64) bool {
	for _, p := range w.players {
		for _, it := range p.Items {
			if it.ID == id {
				return true
			}
		}
	}
	for _, d := range w.drops {
		if d.ID == id {
			return true
		}
	}
	return false
}

// --- Drops y loot ---

func (w *World) rollGold(killer *player, m *mob) int64 {
	if m.Spec.GoldMax <= 0 || w.rng.Float64() >= m.Spec.GoldChance {
		return 0
	}
	minGold, maxGold := m.Spec.GoldMin, m.Spec.GoldMax
	if maxGold < minGold {
		maxGold = minGold
	}
	gold := minGold
	if maxGold > minGold {
		gold += int64(w.rng.Float64() * float64(maxGold-minGold+1))
	}
	if gold < 0 {
		gold = 0
	}
	killer.Gold += gold
	return gold
}

func (w *World) spawnLoot(m *mob) {
	for _, entry := range m.Spec.Loot {
		if w.rng.Float64() >= entry.Chance {
			continue
		}
		tmpl, ok := w.items[entry.Item]
		if !ok {
			continue
		}
		rarity := items.RarityCommon
		if tmpl.Kind == "weapon" || tmpl.Kind == "armor" {
			rarity = items.RollRarity(w.rng.Float64())
		}
		x := clamp(m.X+(w.rng.Float64()*2-1)*0.9, -MapHalfSize, MapHalfSize)
		z := clamp(m.Z+(w.rng.Float64()*2-1)*0.9, -MapHalfSize, MapHalfSize)
		w.spawnDrop(m.MapID, entry.Item, rarity, x, z)
	}
}

func (w *World) spawnDrop(mapID, tmpl, rarity string, x, z float64) {
	d := &drop{
		ID:     w.newItemID(),
		MapID:  mapID,
		Tmpl:   tmpl,
		Rarity: rarity,
		X:      x,
		Z:      z,
		Expire: w.tick + dropLifetimeTicks,
	}
	w.drops = append(w.drops, d)
	w.dropsByID[d.ID] = d
}

func (w *World) removeDrop(d *drop) {
	for i, dd := range w.drops {
		if dd.ID == d.ID {
			w.drops = append(w.drops[:i], w.drops[i+1:]...)
			break
		}
	}
	delete(w.dropsByID, d.ID)
}

func (w *World) simulateDrops() {
	kept := w.drops[:0]
	for _, d := range w.drops {
		if w.tick < d.Expire {
			kept = append(kept, d)
			continue
		}
		delete(w.dropsByID, d.ID)
	}
	w.drops = kept
}

func (w *World) pickup(c Pickup) {
	p, ok := w.players[c.SessionID]
	if !ok || p.HP <= 0 {
		return
	}
	d, ok := w.dropsByID[c.DropID]
	if !ok || d.MapID != p.MapID {
		w.itemError(p, "no_drop", "Ese ítem ya no está en el suelo")
		return
	}
	if math.Hypot(p.X-d.X, p.Z-d.Z) > 3.0 {
		w.itemError(p, "too_far", "Acercate para recoger el ítem")
		return
	}
	if _, ok := w.items[d.Tmpl]; !ok {
		w.removeDrop(d)
		return
	}

	backup := append([]model.Item(nil), p.Items...)
	newItem := model.Item{ID: d.ID, Tmpl: d.Tmpl, Rarity: d.Rarity, Qty: 1, InvX: -1, InvY: -1}
	if !w.addItem(p, newItem) {
		p.Items = backup
		w.itemError(p, "inventory_full", "Inventario lleno")
		return
	}
	w.removeDrop(d)
	w.afterInventoryChange(p)
}

// --- Equipo ---

func (w *World) equipItem(c Equip) {
	p, ok := w.players[c.SessionID]
	if !ok || p.HP <= 0 {
		return
	}
	idx := findItemIndex(p, c.ItemID)
	if idx < 0 {
		w.itemError(p, "invalid_item", "Ítem inexistente")
		return
	}
	if p.Items[idx].EquippedSlot != "" {
		return
	}
	tmpl, ok := w.items[p.Items[idx].Tmpl]
	if !ok || (tmpl.Kind != "weapon" && tmpl.Kind != "armor") || tmpl.Slot == "" {
		w.itemError(p, "not_equippable", "Ese ítem no se puede equipar")
		return
	}
	if !items.CanEquip(tmpl, p.Class, p.stats()) {
		w.itemError(p, "requirements", "No cumplís los requisitos de clase o stats")
		return
	}

	newX, newY := p.Items[idx].InvX, p.Items[idx].InvY
	oldIdx := -1
	for i := range p.Items {
		if i != idx && p.Items[i].EquippedSlot == tmpl.Slot {
			oldIdx = i
			break
		}
	}
	if oldIdx >= 0 {
		p.Items[oldIdx].EquippedSlot = ""
		p.Items[oldIdx].InvX = newX
		p.Items[oldIdx].InvY = newY
	}
	p.Items[idx].EquippedSlot = tmpl.Slot
	p.Items[idx].InvX = -1
	p.Items[idx].InvY = -1
	w.afterInventoryChange(p)
}

func (w *World) unequipItem(c Unequip) {
	p, ok := w.players[c.SessionID]
	if !ok || p.HP <= 0 {
		return
	}
	idx := -1
	for i := range p.Items {
		if p.Items[i].EquippedSlot == c.Slot {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	x, y, ok := w.freeSlot(p)
	if !ok {
		w.itemError(p, "inventory_full", "No hay espacio en el inventario")
		return
	}
	p.Items[idx].EquippedSlot = ""
	p.Items[idx].InvX = x
	p.Items[idx].InvY = y
	w.afterInventoryChange(p)
}

func (w *World) moveItem(c InvMove) {
	p, ok := w.players[c.SessionID]
	if !ok || p.HP <= 0 {
		return
	}
	if c.X < 0 || c.X >= inventoryWidth || c.Y < 0 || c.Y >= inventoryHeight {
		return
	}
	idx := findItemIndex(p, c.ItemID)
	if idx < 0 || p.Items[idx].EquippedSlot != "" {
		return
	}
	if p.Items[idx].InvX == c.X && p.Items[idx].InvY == c.Y {
		return
	}
	other := w.itemAt(p, c.X, c.Y)
	if other == idx {
		return
	}
	if other >= 0 {
		p.Items[other].InvX, p.Items[other].InvY = p.Items[idx].InvX, p.Items[idx].InvY
	}
	p.Items[idx].InvX = c.X
	p.Items[idx].InvY = c.Y
	w.afterInventoryChange(p)
}

// --- Consumibles ---

func (w *World) usePotion(c Potion) {
	p, ok := w.players[c.SessionID]
	if !ok {
		return
	}
	for _, it := range p.Items {
		if it.EquippedSlot != "" {
			continue
		}
		tmpl, ok := w.items[it.Tmpl]
		if !ok || tmpl.Kind != "potion" || tmpl.HealKind != c.Kind {
			continue
		}
		w.useItem(c.SessionID, it.ID)
		return
	}
}

func (w *World) useItem(sessionID, itemID int64) {
	p, ok := w.players[sessionID]
	if !ok || p.HP <= 0 {
		return
	}
	if w.tick < p.NextPotionTick {
		return
	}
	idx := findItemIndex(p, itemID)
	if idx < 0 || p.Items[idx].EquippedSlot != "" {
		return
	}
	tmpl, ok := w.items[p.Items[idx].Tmpl]
	if !ok || tmpl.Kind != "potion" {
		w.itemError(p, "not_usable", "Ese ítem no se puede usar")
		return
	}

	switch tmpl.HealKind {
	case "hp":
		heal := max(1, p.HPMax*tmpl.HealPercent/100)
		p.HP = min(p.HPMax, p.HP+heal)
	case "mp":
		heal := max(1, p.MPMax*tmpl.HealPercent/100)
		p.MP = min(p.MPMax, p.MP+heal)
	default:
		return
	}

	p.NextPotionTick = w.tick + potionCooldownTicks
	p.Items[idx].Qty--
	if p.Items[idx].Qty <= 0 {
		w.removeItemAt(p, idx)
	}
	w.afterInventoryChange(p)
}

// --- Mejora +N ---

func (w *World) enhanceItem(c Enhance) {
	p, ok := w.players[c.SessionID]
	if !ok || p.HP <= 0 {
		return
	}
	idx := findItemIndex(p, c.ItemID)
	if idx < 0 {
		w.itemError(p, "invalid_item", "Ítem inexistente")
		return
	}
	tmpl, ok := w.items[p.Items[idx].Tmpl]
	if !ok || tmpl.Slot == "" {
		w.itemError(p, "not_enhanceable", "Ese ítem no se puede mejorar")
		return
	}
	from := p.Items[idx].Enhance
	if from >= items.MaxEnhance {
		w.itemError(p, "enhance_max", "El ítem ya está en +12")
		return
	}

	coreIdx := -1
	for i := range p.Items {
		if p.Items[i].EquippedSlot != "" {
			continue
		}
		t, ok := w.items[p.Items[i].Tmpl]
		if ok && t.Kind == "core" && p.Items[i].Qty > 0 {
			coreIdx = i
			break
		}
	}
	if coreIdx < 0 {
		w.itemError(p, "no_core", "Necesitás un Núcleo de mejora")
		return
	}

	cost := items.EnhanceCost(from)
	if p.Gold < cost {
		w.itemError(p, "no_gold", "No tenés oro suficiente")
		return
	}

	// Consumir material y oro.
	p.Items[coreIdx].Qty--
	if p.Items[coreIdx].Qty <= 0 {
		w.removeItemAt(p, coreIdx)
	}
	p.Gold -= cost

	newLevel, success, destroyed := items.ResolveEnhance(from, w.rng.Float64(), w.rng.Float64())

	// Re-buscar: el índice pudo cambiar al borrar el núcleo.
	idx = findItemIndex(p, c.ItemID)
	if idx >= 0 {
		if destroyed {
			w.removeItemAt(p, idx)
		} else {
			p.Items[idx].Enhance = newLevel
		}
	}

	if data, err := protocol.Encode("enhance_result", protocol.EnhanceResult{
		ItemID: c.ItemID, Success: success, NewEnhance: newLevel, Destroyed: destroyed,
	}); err == nil {
		w.send(p, data)
	}
	w.log.Info("intento de mejora",
		"player", p.Name, "item", tmpl.ID, "from", from, "cost", cost,
		"success", success, "destroyed", destroyed, "new", newLevel)

	w.afterInventoryChange(p)
}

// --- Tienda ---

func (w *World) shopBuy(c ShopBuy) {
	p, ok := w.players[c.SessionID]
	if !ok || p.HP <= 0 {
		return
	}
	tmpl, ok := w.items[c.Tmpl]
	if !ok || tmpl.BuyPrice <= 0 {
		w.itemError(p, "not_for_sale", "Ese ítem no está a la venta")
		return
	}
	qty := c.Qty
	if qty < 1 {
		qty = 1
	}
	if !tmpl.Stackable {
		qty = 1
	}
	if qty > 20 {
		qty = 20
	}
	total := tmpl.BuyPrice * int64(qty)
	if p.Gold < total {
		w.itemError(p, "no_gold", "No tenés oro suficiente")
		return
	}

	backup := append([]model.Item(nil), p.Items...)
	if !w.addItem(p, model.Item{Tmpl: tmpl.ID, Rarity: items.RarityCommon, Qty: qty}) {
		p.Items = backup
		w.itemError(p, "inventory_full", "Inventario lleno")
		return
	}
	p.Gold -= total
	w.afterInventoryChange(p)
}

func (w *World) shopSell(c ShopSell) {
	p, ok := w.players[c.SessionID]
	if !ok || p.HP <= 0 {
		return
	}
	idx := findItemIndex(p, c.ItemID)
	if idx < 0 {
		return
	}
	if p.Items[idx].EquippedSlot != "" {
		w.itemError(p, "equipped", "Desequipá el ítem antes de venderlo")
		return
	}
	tmpl, ok := w.items[p.Items[idx].Tmpl]
	if !ok {
		return
	}
	price := items.SellPrice(tmpl, p.Items[idx].Rarity, p.Items[idx].Enhance) * int64(p.Items[idx].Qty)
	p.Gold += price
	w.removeItemAt(p, idx)
	w.afterInventoryChange(p)
}
