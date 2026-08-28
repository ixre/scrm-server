package models

import (
	"core/app/constants"
	"time"
)

// CrmProductCategory 产品分类。
type CrmProductCategory struct {
	Model
	Timestamp
	TenantID string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Name     string `gorm:"type:varchar(128);comment:分类名称" json:"name"`
	Sort     int    `gorm:"type:int;default:0;comment:排序" json:"sort"`
	Status   bool   `gorm:"type:bool;default:true;comment:是否启用" json:"status"`
	Remark   string `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmProduct 产品档案。
type CrmProduct struct {
	Model
	Timestamp
	TenantID   string  `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Name       string  `gorm:"type:varchar(255);comment:产品名称" json:"name"`
	Category   string  `gorm:"type:varchar(64);comment:分类(冗余便于展示)" json:"category"`
	CategoryID string  `gorm:"type:bigint;index;comment:产品分类ID(关联CrmProductCategory)" json:"category_id"`
	Spec       string  `gorm:"type:varchar(255);comment:规格" json:"spec"`
	Unit       string  `gorm:"type:varchar(32);comment:单位" json:"unit"`
	Price      float64 `gorm:"type:decimal(16,2);comment:售价" json:"price"`
	Cost       float64 `gorm:"type:decimal(16,2);comment:成本价" json:"cost"`
	SKU        string  `gorm:"type:varchar(64);index;comment:SKU编码" json:"sku"`
	Status     string  `gorm:"type:varchar(16);default:active;comment:active/disabled" json:"status"`
	Remark     string  `gorm:"type:varchar(512);comment:备注" json:"remark"`
	ProductNo  string  `gorm:"type:varchar(32);index;comment:产品编号" json:"product_no"`
	StockQty   int     `gorm:"type:int;default:0;comment:库存数量" json:"stock_qty"`
	MinStock   int     `gorm:"type:int;default:0;comment:最小库存(预警下限)" json:"min_stock"`
}

// CrmWarehouse 仓库。
type CrmWarehouse struct {
	Model
	Timestamp
	TenantID    string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Name        string `gorm:"type:varchar(128);comment:仓库名称" json:"name"`
	Address     string `gorm:"type:varchar(255);comment:仓库地址" json:"address"`
	Manager     string `gorm:"type:bigint;comment:仓库管理员" json:"manager"`
	Status      string `gorm:"type:varchar(16);default:active;comment:active/disabled" json:"status"`
	WarehouseNo string `gorm:"type:varchar(32);index;comment:仓库编号" json:"warehouse_no"`
}

// CrmStockRecord 库存出入库记录。
type CrmStockRecord struct {
	Model
	Timestamp
	TenantID    string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	WarehouseID string     `gorm:"type:bigint;index;comment:仓库ID" json:"warehouse_id"`
	ProductID   string     `gorm:"type:bigint;index;comment:产品ID" json:"product_id"`
	Type        string     `gorm:"type:varchar(16);index;comment:类型 instock(入库)/outstock(出库)/transfer(调拨)/check(盘点)" json:"type"`
	Qty         int        `gorm:"type:int;comment:数量(正入库/负出库)" json:"qty"`
	OrderNo     string     `gorm:"type:varchar(64);comment:关联单号" json:"order_no"`
	OccurAt     *time.Time `gorm:"comment:发生时间" json:"occur_at"`
	OperatorID  string     `gorm:"type:bigint;comment:操作人" json:"operator_id"`
	Remark      string     `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmSupplier 供应商。
type CrmSupplier struct {
	Model
	Timestamp
	TenantID string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Name     string `gorm:"type:varchar(255);comment:供应商名称" json:"name"`
	Contact  string `gorm:"type:varchar(64);comment:联系人" json:"contact"`
	Phone    string `gorm:"type:varchar(32);comment:电话" json:"phone"`
	Address  string `gorm:"type:varchar(255);comment:地址" json:"address"`
	Remark   string `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmPurchaseOrder 采购订单。
type CrmPurchaseOrder struct {
	Model
	Timestamp
	TenantID    string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	No          string     `gorm:"type:varchar(32);index;comment:采购单号" json:"no"`
	SupplierID  string     `gorm:"type:bigint;index;comment:供应商ID" json:"supplier_id"`
	WarehouseID string     `gorm:"type:bigint;comment:入库仓库ID" json:"warehouse_id"`
	Amount      float64    `gorm:"type:decimal(16,2);comment:采购金额" json:"amount"`
	PaidAmount  float64    `gorm:"type:decimal(16,2);default:0;comment:已付金额" json:"paid_amount"`
	Status      string     `gorm:"type:varchar(16);default:draft;comment:draft/ordered/instocked/paid/cancelled" json:"status"`
	OrderDate   *time.Time `gorm:"comment:下单日期" json:"order_date"`
	Remark      string     `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// ==================== 进销存单据 ====================

// CrmStockIn 入库单。
type CrmStockIn struct {
	Model
	Timestamp
	TenantID    string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	No          string                     `gorm:"type:varchar(32);index;comment:入库单号" json:"no"`
	WarehouseID string                     `gorm:"type:bigint;index;comment:仓库ID" json:"warehouse_id"`
	Type        string                     `gorm:"type:varchar(16);comment:purchase/return/transfer_in/other" json:"type"`
	RefNo       string                     `gorm:"type:varchar(64);comment:关联单号(采购单号等)" json:"ref_no"`
	TotalAmount float64                    `gorm:"type:decimal(16,2);comment:总金额" json:"total_amount"`
	OccurDate   *time.Time                 `gorm:"comment:入库日期" json:"occur_date"`
	OperatorID  string                     `gorm:"type:bigint;comment:操作人" json:"operator_id"`
	Status      string                     `gorm:"type:varchar(16);default:draft;comment:draft/confirmed/cancelled" json:"status"`
	Remark      string                     `gorm:"type:varchar(512);comment:备注" json:"remark"`
	Items       constants.StringArrayField `gorm:"type:jsonb;comment:明细JSON[{product_id,name,spec,qty,price,amount}]" json:"items"`
}

// CrmStockOut 出库单。
type CrmStockOut struct {
	Model
	Timestamp
	TenantID    string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	No          string                     `gorm:"type:varchar(32);index;comment:出库单号" json:"no"`
	WarehouseID string                     `gorm:"type:bigint;index;comment:仓库ID" json:"warehouse_id"`
	Type        string                     `gorm:"type:varchar(16);comment:sales/return/transfer_out/other" json:"type"`
	RefNo       string                     `gorm:"type:varchar(64);comment:关联单号" json:"ref_no"`
	TotalAmount float64                    `gorm:"type:decimal(16,2);comment:总金额" json:"total_amount"`
	OccurDate   *time.Time                 `gorm:"comment:出库日期" json:"occur_date"`
	OperatorID  string                     `gorm:"type:bigint;comment:操作人" json:"operator_id"`
	Status      string                     `gorm:"type:varchar(16);default:draft;comment:draft/confirmed/cancelled" json:"status"`
	Remark      string                     `gorm:"type:varchar(512);comment:备注" json:"remark"`
	Items       constants.StringArrayField `gorm:"type:jsonb;comment:明细JSON" json:"items"`
}

// CrmStockCheck 库存盘点单。
type CrmStockCheck struct {
	Model
	Timestamp
	TenantID    string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	No          string                     `gorm:"type:varchar(32);index;comment:盘点单号" json:"no"`
	WarehouseID string                     `gorm:"type:bigint;index;comment:仓库ID" json:"warehouse_id"`
	OccurDate   *time.Time                 `gorm:"comment:盘点日期" json:"occur_date"`
	OperatorID  string                     `gorm:"type:bigint;comment:操作人" json:"operator_id"`
	Status      string                     `gorm:"type:varchar(16);default:draft;comment:draft/confirmed/cancelled" json:"status"`
	Remark      string                     `gorm:"type:varchar(512);comment:备注" json:"remark"`
	Items       constants.StringArrayField `gorm:"type:jsonb;comment:盘点明细JSON[{product_id,name,book_qty,actual_qty,diff}]" json:"items"`
}

// CrmStockTransfer 库存调拨单。
type CrmStockTransfer struct {
	Model
	Timestamp
	TenantID      string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	No            string                     `gorm:"type:varchar(32);index;comment:调拨单号" json:"no"`
	FromWarehouse string                     `gorm:"type:bigint;index;comment:调出仓库ID" json:"from_warehouse"`
	ToWarehouse   string                     `gorm:"type:bigint;index;comment:调入仓库ID" json:"to_warehouse"`
	OccurDate     *time.Time                 `gorm:"comment:调拨日期" json:"occur_date"`
	OperatorID    string                     `gorm:"type:bigint;comment:操作人" json:"operator_id"`
	Status        string                     `gorm:"type:varchar(16);default:draft;comment:draft/confirmed/cancelled" json:"status"`
	Remark        string                     `gorm:"type:varchar(512);comment:备注" json:"remark"`
	Items         constants.StringArrayField `gorm:"type:jsonb;comment:调拨明细JSON" json:"items"`
}
