package protocol

import "munext/server/internal/model"

// Mensajes de F4: loot, inventario, equipamiento, tienda y mejora +N.

// Pickup recoge un drop del suelo.
type Pickup struct {
	ID int64 `json:"id"`
}

// Equip equipa un ítem del inventario.
type Equip struct {
	ItemID int64 `json:"item_id"`
}

// Unequip desequipa el ítem de un slot.
type Unequip struct {
	Slot string `json:"slot"`
}

// InvMove mueve un ítem dentro de la grilla 8x5.
type InvMove struct {
	ItemID int64 `json:"item_id"`
	X      int   `json:"x"`
	Y      int   `json:"y"`
}

// UseItem usa una poción del inventario.
type UseItem struct {
	ItemID int64 `json:"item_id"`
}

// Enhance intenta mejorar un ítem (+N).
type Enhance struct {
	ItemID int64 `json:"item_id"`
}

// ShopBuy compra un consumible del vendedor.
type ShopBuy struct {
	Tmpl string `json:"tmpl"`
	Qty  int    `json:"qty"`
}

// ShopSell vende un ítem del inventario.
type ShopSell struct {
	ItemID int64 `json:"item_id"`
}

// Inventory es el estado completo del inventario y el oro.
type Inventory struct {
	Gold  int64        `json:"gold"`
	Items []model.Item `json:"items"`
}

// EnhanceResult informa el resultado de una mejora (para el momento de tensión).
type EnhanceResult struct {
	ItemID     int64 `json:"item_id"`
	Success    bool  `json:"success"`
	NewEnhance int   `json:"new_enhance"`
	Destroyed  bool  `json:"destroyed"`
}
