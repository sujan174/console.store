package main
import("context";"encoding/json";"fmt";"os";"github.com/jackc/pgx/v5/pgxpool";"console.store/internal/store";"console.store/internal/store/kms";"console.store/internal/swiggy")
func main(){
	ctx:=context.Background()
	pool,_:=pgxpool.New(ctx,os.Getenv("CONSOLE_DB_DSN"));defer pool.Close()
	k,_:=kms.FromEnv(ctx)
	tok,_,_:=store.New(pool,k).GetToken(ctx,"47e0c13e-17cc-419a-a209-53ed94fc41f8")
	c:=swiggy.NewClient(swiggy.FoodBaseURL,swiggy.StaticToken(tok.AccessToken))
	addrs:=[]struct{ID,Tag string}{{"362472411","Home"},{"ci9tkhpfq5t4ec8ieu40","Basketball Court"},{"cqdtp1pormb52smbqiug","Post Office Gate"},{"ckfa3o9ndaoein2r6tb0","Work"}}
	for _,a:=range addrs{
		for _,q:=range []string{"mocha","cafe mocha"}{
			raw,_:=c.CallTool(ctx,"search_menu",map[string]any{"addressId":a.ID,"query":q,"offset":0})
			var r struct{Total int `json:"total"`;Items []struct{
				ID string `json:"id"`;Name string `json:"name"`;RestaurantName string `json:"restaurantName"`;HasAddons bool `json:"hasAddons"`;HasVariants bool `json:"hasVariants"`;Price float64 `json:"price"`
			}`json:"items"`}
			json.Unmarshal(raw,&r)
			if r.Total>0{
				fmt.Printf("[%s] q=%q total=%d\n",a.Tag,q,r.Total)
				for i,it:=range r.Items{if i>=4{break};fmt.Printf("    %s @ %s Rs%.0f A=%v V=%v (id=%s)\n",it.Name,it.RestaurantName,it.Price,it.HasAddons,it.HasVariants,it.ID)}
			}
		}
	}
	fmt.Println("done")
}
