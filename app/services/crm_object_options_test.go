package services

import (
	"core/app/constants"
	"core/app/models"
	"core/migrations"
	"fmt"
	"strings"
	"testing"
	"gorm.io/gorm"
)

func objectOptionsDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := regressionDB(t)
	ddl, err := migrations.BusinessOptionIndexStatements()
	requireNoError(t, err)
	for i := 0; i < 2; i++ {
		for _, stmt := range ddl { requireNoError(t, db.Exec(stmt).Error) }
	}
	return db
}

func TestObjectOptionsBeyond500AndLiteralSearch(t *testing.T) {
	db := objectOptionsDB(t)
	var customers []models.CrmCustomer
	var orders []models.CrmOrder
	for i := 0; i < 605; i++ {
		id := fmt.Sprint(1000+i)
		customers = append(customers, models.CrmCustomer{Model: models.Model{ID: id}, TenantID: "1", OwnerUserID: "10", Name: fmt.Sprintf("client-%04d",i), Company: "Example", Mobile: fmt.Sprintf("138%08d",i)})
		orders = append(orders, models.CrmOrder{Model: models.Model{ID: id}, TenantID: "1", CustomerID: "100", UserID: "10", Title: fmt.Sprintf("contract-%04d",i), OrderNo: "SO"+id, Status: "signed"})
	}
	requireNoError(t, db.Omit("ReassignFrom").CreateInBatches(&customers,100).Error)
	requireNoError(t, db.Omit("OpportunityID","CouponID","ProjectID","WorkorderID").CreateInBatches(&orders,100).Error)
	ctx := testIdentity("1","10")
	for _, kind := range []string{"customer","order"} {
		seen := map[string]bool{}
		cursor := ""
		for {
			page, err := objectOptions(ctx,kind,ObjectOptionsRequest{Cursor:cursor,PageSize:1000})
			requireNoError(t,err)
			if len(page.Items)>100 {t.Fatal("unbounded options")}
			for _, row := range page.Items {
				if seen[row.ID] {t.Fatal("cursor repeats an item")}; seen[row.ID]=true
			}
			if !page.HasMore {break}; cursor=page.NextCursor
		}
		want:=605; if kind=="customer" {want++}
		if len(seen)!=want || !seen["1000"] || !seen["1604"] {t.Fatalf("missing options: %s %d",kind,len(seen))}
		for _, keyword := range []string{"%","_",`\`,"missing"} {
			page,err:=objectOptions(ctx,kind,ObjectOptionsRequest{Keyword:keyword}); requireNoError(t,err)
			if len(page.Items)!=0 {t.Fatal("wildcard expanded search")}
		}
		keyword:="CLIENT-0000"; if kind=="order" {keyword="CONTRACT-0000"}
		page,err:=objectOptions(ctx,kind,ObjectOptionsRequest{Keyword:keyword}); requireNoError(t,err)
		if len(page.Items)!=1 || page.Items[0].ID!="1000" {t.Fatal("older item not searchable")}
		page,err=objectOptions(ctx,kind,ObjectOptionsRequest{Keyword:"1000"}); requireNoError(t,err)
		if len(page.Items)!=1 || page.Items[0].ID!="1000" {t.Fatal("exact ID search failed")}
		page,err=objectOptions(ctx,kind,ObjectOptionsRequest{IDs:[]string{"1000","1000","1604","102"}}); requireNoError(t,err)
		if len(page.Items)!=2 || page.HasMore {t.Fatal("selected resolution leaks foreign data or ignores older IDs")}
	}
	page,err:=NewCrmCustomer().Options(ctx,ObjectOptionsRequest{Keyword:"13800000000"});requireNoError(t,err)
	if len(page.Items)!=1 {t.Fatal("phone prefix search failed")}
	page,err=NewCrmOrder().Options(ctx,ObjectOptionsRequest{Keyword:"so1000"});requireNoError(t,err)
	if len(page.Items)!=1 {t.Fatal("contract number search failed")}
}

func TestObjectOptionsAccessAndEligibility(t *testing.T) {
	db:=objectOptionsDB(t)
	ctx:=testIdentity("1","10")
	requireNoError(t,db.Model(&models.CrmCustomer{}).Where("id=101").Update("shared_staff_ids",constants.StringArrayField{"10"}).Error)
	for _, row:=range []models.CrmCustomer{
		{Model:models.Model{ID:"103"},TenantID:"1",OwnerUserID:"10",Name:"Pool",PoolStatus:1},
		{Model:models.Model{ID:"104"},TenantID:"1",OwnerUserID:"10",Name:"Pending",ReassignStatus:1},
		{Model:models.Model{ID:"105"},TenantID:"1",Name:"Unowned"},
	} { copy:=row;mustCreate(t,db,&copy) }
	read,err:=NewCrmCustomer().Options(ctx,ObjectOptionsRequest{});requireNoError(t,err)
	if len(read.Items)!=4 {t.Fatal("read boundary differs from customer reads")}
	sales,err:=NewCrmCustomer().Options(ctx,ObjectOptionsRequest{Purpose:"sales"});requireNoError(t,err)
	if len(sales.Items)!=1 || sales.Items[0].ID!="100" {t.Fatal("shared, pool or pending customer assignable")}
	selected,err:=NewCrmCustomer().Options(ctx,ObjectOptionsRequest{Purpose:"sales",IDs:[]string{"100","101","102","103","104","105"}});requireNoError(t,err)
	if len(selected.Items)!=4 {t.Fatal("historical labels not read-scoped")}
	for _, row:=range selected.Items {if row.Disabled!=(row.ID!="100"){t.Fatal("historical selection grants write eligibility")}}
	admin,err:=NewCrmCustomer().Options(testIdentity("1","12"),ObjectOptionsRequest{Purpose:"sales"});requireNoError(t,err)
	if len(admin.Items)!=2 {t.Fatal("manager cannot select another owner's active customer")}
	for _, row:=range []models.CrmOrder{
		{Model:models.Model{ID:"2000"},TenantID:"1",CustomerID:"101",UserID:"10",OrderNo:"A",Status:"signed"},
		{Model:models.Model{ID:"2001"},TenantID:"1",CustomerID:"100",UserID:"11",OrderNo:"B",Status:"signed"},
		{Model:models.Model{ID:"2002"},TenantID:"1",CustomerID:"100",UserID:"10",OrderNo:"C",Status:"cancelled"},
		{Model:models.Model{ID:"2003"},TenantID:"2",CustomerID:"102",UserID:"20",OrderNo:"D",Status:"signed"},
	} { copy:=row;mustCreate(t,db,&copy) }
	plan,err:=NewCrmOrder().Options(ctx,ObjectOptionsRequest{Purpose:"plan"});requireNoError(t,err)
	if len(plan.Items)!=1 || plan.Items[0].ID!="2000" || plan.Items[0].CustomerID!="101" {t.Fatal("contract access incorrectly depends on customer ownership or sharing")}
	selected,err=NewCrmOrder().Options(ctx,ObjectOptionsRequest{Purpose:"plan",IDs:[]string{"2000","2001","2002","2003"}});requireNoError(t,err)
	if len(selected.Items)!=2 || !selected.Items[0].Disabled || selected.Items[1].Disabled {t.Fatal("cancelled contract not disabled or foreign contract leaked")}
	requireNoError(t,db.Model(&models.CrmCustomer{}).Where("id=100").Update("owner_user_id","11").Error)
	requireNoError(t,db.Model(&models.CrmOrder{}).Where("id=2000").Update("user_id","11").Error)
	for _, kind:=range []string{"customer","order"} {
		id:="100";if kind=="order"{id="2000"}
		page,err:=objectOptions(ctx,kind,ObjectOptionsRequest{IDs:[]string{id}});requireNoError(t,err)
		if len(page.Items)!=0{t.Fatal("revoked selected value still readable")}
	}
	if _,err=objectOptions(testIdentity("2","10"),"customer",ObjectOptionsRequest{});err==nil{t.Fatal("forged actor accepted")}
	requireNoError(t,db.Model(&models.User{}).Where("id=10").Update("status",2).Error)
	if _,err=objectOptions(ctx,"order",ObjectOptionsRequest{});err==nil{t.Fatal("disabled actor accepted")}
}

func TestObjectOptionsCursorBindingAndValidation(t *testing.T) {
	db:=objectOptionsDB(t)
	requireNoError(t,db.Model(&models.CrmCustomer{}).Where("id=101").Update("owner_user_id","10").Error)
	ctx:=testIdentity("1","10")
	first,err:=objectOptions(ctx,"customer",ObjectOptionsRequest{PageSize:1});requireNoError(t,err)
	if !first.HasMore {t.Fatal("missing cursor")}
	for _, req:=range []ObjectOptionsRequest{
		{Cursor:"invalid"},{Cursor:strings.Repeat("x",2049)},{Cursor:first.NextCursor,Keyword:"changed"},
		{Cursor:first.NextCursor,Purpose:"sales"},{Purpose:"plan"},{Keyword:strings.Repeat("字",129)},
		{Keyword:"\x00"},{IDs:[]string{"-1"}},{IDs:[]string{"1.0"}},{IDs:[]string{"01"}},
		{IDs:[]string{"100"},Cursor:first.NextCursor},{IDs:make([]string,101)},
	} {if _,err=objectOptions(ctx,"customer",req);err==nil {t.Fatalf("invalid input accepted: %+v",req)}}
	if _,err=objectOptions(ctx,"order",ObjectOptionsRequest{Cursor:first.NextCursor});err==nil{t.Fatal("cursor reused across objects")}
	if _,err=objectOptions(testIdentity("1","12"),"customer",ObjectOptionsRequest{Cursor:first.NextCursor});err==nil{t.Fatal("cursor reused by other actor")}
	requireNoError(t,db.Model(&models.CrmCustomer{}).Where("id=100").Update("owner_user_id","11").Error)
	next,err:=objectOptions(ctx,"customer",ObjectOptionsRequest{Cursor:first.NextCursor});requireNoError(t,err)
	if len(next.Items)!=0 {t.Fatal("cursor bypassed current permissions")}
	// A new high ID must not shift earlier pages.
	mustCreate(t,db,&models.CrmCustomer{Model:models.Model{ID:"2000"},TenantID:"1",OwnerUserID:"10",Name:"new"})
	next,err=objectOptions(ctx,"customer",ObjectOptionsRequest{Cursor:first.NextCursor});requireNoError(t,err)
	if len(next.Items)!=0 {t.Fatal("new insertion shifted cursor")}
}
