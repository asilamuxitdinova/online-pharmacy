package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

// Every value below is made up. Nothing here belongs to a real person.

type SeedUser struct {
	ID, Email, Phone, Name, Password, Role, TIN, APIKey string
	Disabled                                            bool
}

// SeedUsers is one account per role, plus a second customer (to prove that
// someone else's records answer 404) and a disabled one (403).
var SeedUsers = []SeedUser{
	{ID: "usr_aziza", Email: "aziza@pharmacy.test", Phone: "+998900000001", Name: "Aziza Karimova", Password: "Customer123!", Role: "customer", TIN: "301234567", APIKey: "phk_test_customer_aziza_0000000000000001"},
	{ID: "usr_bekzod", Email: "bekzod@pharmacy.test", Phone: "+998900000002", Name: "Bekzod Rahimov", Password: "Customer123!", Role: "customer", TIN: "302345678", APIKey: "phk_test_customer_bekzod_000000000000002"},
	{ID: "usr_dilnoza", Email: "dilnoza@pharmacy.test", Phone: "+998900000003", Name: "Dilnoza Yusupova", Password: "Staff123!", Role: "staff", TIN: "303456789", APIKey: "phk_test_staff_dilnoza_00000000000000003"},
	{ID: "usr_admin", Email: "admin@pharmacy.test", Phone: "+998900000004", Name: "Admin", Password: "Admin123!", Role: "admin", TIN: "304567890", APIKey: "phk_test_admin_000000000000000000000004"},
	{ID: "usr_timur", Email: "timur@pharmacy.test", Phone: "+998900000005", Name: "Timur Saidov", Password: "Customer123!", Role: "customer", TIN: "305678901", Disabled: true},
}

var (
	hashOnce sync.Once
	hashes   map[string]string
)

// passwordHash caches bcrypt hashes so a reset (run before every test) does
// not pay for bcrypt five times.
func passwordHash(pw string) string {
	hashOnce.Do(func() {
		hashes = map[string]string{}
		for _, u := range SeedUsers {
			if _, ok := hashes[u.Password]; ok {
				continue
			}
			h, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
			if err != nil {
				panic(err)
			}
			hashes[u.Password] = string(h)
		}
	})
	return hashes[pw]
}

type category struct{ id, en, ru, uz string }

var categories = []category{
	{"pain-relief", "Pain relief", "Обезболивающие", "Og'riq qoldiruvchilar"},
	{"cold-flu", "Cold and flu", "Простуда и грипп", "Shamollash va gripp"},
	{"antibiotics", "Antibiotics", "Антибиотики", "Antibiotiklar"},
	{"allergy", "Allergy", "Аллергия", "Allergiya"},
	{"digestion", "Digestion", "Пищеварение", "Hazm qilish"},
	{"vitamins", "Vitamins and supplements", "Витамины и добавки", "Vitaminlar va qo'shimchalar"},
	{"heart", "Heart and blood pressure", "Сердце и давление", "Yurak va qon bosimi"},
	{"diabetes", "Diabetes", "Диабет", "Diabet"},
	{"skin", "Skin care", "Уход за кожей", "Teri parvarishi"},
	{"eye-ear", "Eye and ear", "Глаза и уши", "Ko'z va quloq"},
	{"first-aid", "First aid", "Первая помощь", "Birinchi yordam"},
	{"baby", "Mother and baby", "Мама и малыш", "Ona va bola"},
}

type form struct {
	en, ru, uz string
	liquid     bool
	packs      []int
}

var forms = map[string]form{
	"tablets":  {"tablets", "таблеток", "ta tabletka", false, []int{10, 20, 30}},
	"capsules": {"capsules", "капсул", "ta kapsula", false, []int{10, 20, 30}},
	"sachets":  {"sachets", "саше", "ta paket", false, []int{6, 10, 20}},
	"syrup":    {"ml syrup", "мл сироп", "ml sirop", true, []int{100, 200}},
	"drops":    {"ml drops", "мл капли", "ml tomchi", true, []int{10, 15}},
	"cream":    {"g cream", "г крем", "g krem", true, []int{15, 30, 50}},
	"spray":    {"ml spray", "мл спрей", "ml sprey", true, []int{20, 30}},
}

type medicine struct {
	en, ru, uz, category, form string
	rx                         bool
	price                      int64 // UZS for the smallest pack of the first strength
	strengths                  []string
}

var medicines = []medicine{
	{"Paracetamol", "Парацетамол", "Paratsetamol", "pain-relief", "tablets", false, 4_000, []string{"500 mg", "250 mg"}},
	{"Ibuprofen", "Ибупрофен", "Ibuprofen", "pain-relief", "tablets", false, 9_000, []string{"200 mg", "400 mg"}},
	{"Ibuprofen for children", "Ибупрофен детский", "Bolalar uchun ibuprofen", "baby", "syrup", false, 24_000, []string{"100 mg/5 ml"}},
	{"Diclofenac", "Диклофенак", "Diklofenak", "pain-relief", "cream", false, 18_000, []string{"1%", "5%"}},
	{"Naproxen", "Напроксен", "Naproksen", "pain-relief", "tablets", false, 21_000, []string{"250 mg", "500 mg"}},
	{"Tramadol", "Трамадол", "Tramadol", "pain-relief", "capsules", true, 38_000, []string{"50 mg"}},
	{"Aspirin", "Аспирин", "Aspirin", "heart", "tablets", false, 7_000, []string{"100 mg", "500 mg"}},
	{"Amoxicillin", "Амоксициллин", "Amoksitsillin", "antibiotics", "capsules", true, 26_000, []string{"250 mg", "500 mg"}},
	{"Azithromycin", "Азитромицин", "Azitromitsin", "antibiotics", "tablets", true, 42_000, []string{"250 mg", "500 mg"}},
	{"Ciprofloxacin", "Ципрофлоксацин", "Siprofloksatsin", "antibiotics", "tablets", true, 31_000, []string{"250 mg", "500 mg"}},
	{"Cefixime", "Цефиксим", "Sefiksim", "antibiotics", "capsules", true, 58_000, []string{"200 mg", "400 mg"}},
	{"Doxycycline", "Доксициклин", "Doksitsiklin", "antibiotics", "capsules", true, 22_000, []string{"100 mg"}},
	{"Cetirizine", "Цетиризин", "Setirizin", "allergy", "tablets", false, 12_000, []string{"10 mg"}},
	{"Loratadine", "Лоратадин", "Loratadin", "allergy", "tablets", false, 10_000, []string{"10 mg"}},
	{"Desloratadine", "Дезлоратадин", "Desloratadin", "allergy", "syrup", false, 47_000, []string{"0.5 mg/ml"}},
	{"Mometasone nasal", "Мометазон назальный", "Mometazon burun uchun", "allergy", "spray", true, 89_000, []string{"50 mcg"}},
	{"Oseltamivir", "Осельтамивир", "Oseltamivir", "cold-flu", "capsules", true, 145_000, []string{"75 mg", "30 mg"}},
	{"Cold and flu powder", "Порошок от простуды", "Shamollashga qarshi kukun", "cold-flu", "sachets", false, 29_000, []string{"lemon", "honey"}},
	{"Ambroxol", "Амброксол", "Ambroksol", "cold-flu", "syrup", false, 19_000, []string{"15 mg/5 ml", "30 mg/5 ml"}},
	{"Xylometazoline", "Ксилометазолин", "Ksilometazolin", "cold-flu", "spray", false, 16_000, []string{"0.05%", "0.1%"}},
	{"Throat lozenges", "Леденцы для горла", "Tomoq uchun pastilkalar", "cold-flu", "tablets", false, 14_000, []string{"menthol", "honey"}},
	{"Omeprazole", "Омепразол", "Omeprazol", "digestion", "capsules", false, 15_000, []string{"20 mg", "40 mg"}},
	{"Pantoprazole", "Пантопразол", "Pantoprazol", "digestion", "tablets", true, 34_000, []string{"20 mg", "40 mg"}},
	{"Loperamide", "Лоперамид", "Loperamid", "digestion", "capsules", false, 8_000, []string{"2 mg"}},
	{"Oral rehydration salts", "Регидрационная соль", "Regidratatsion tuz", "digestion", "sachets", false, 11_000, []string{"standard"}},
	{"Activated charcoal", "Активированный уголь", "Faollashtirilgan ko'mir", "digestion", "tablets", false, 3_000, []string{"250 mg"}},
	{"Probiotic", "Пробиотик", "Probiotik", "digestion", "capsules", false, 52_000, []string{"10 bn CFU"}},
	{"Vitamin C", "Витамин C", "Vitamin C", "vitamins", "tablets", false, 13_000, []string{"500 mg", "1000 mg"}},
	{"Vitamin D3", "Витамин D3", "Vitamin D3", "vitamins", "drops", false, 36_000, []string{"500 IU", "1000 IU"}},
	{"Magnesium B6", "Магний B6", "Magniy B6", "vitamins", "tablets", false, 41_000, []string{"470 mg"}},
	{"Iron supplement", "Препарат железа", "Temir preparati", "vitamins", "tablets", false, 33_000, []string{"100 mg"}},
	{"Folic acid", "Фолиевая кислота", "Foliy kislotasi", "baby", "tablets", false, 6_000, []string{"1 mg", "5 mg"}},
	{"Amlodipine", "Амлодипин", "Amlodipin", "heart", "tablets", true, 17_000, []string{"5 mg", "10 mg"}},
	{"Lisinopril", "Лизиноприл", "Lizinopril", "heart", "tablets", true, 19_000, []string{"10 mg", "20 mg"}},
	{"Atorvastatin", "Аторвастатин", "Atorvastatin", "heart", "tablets", true, 46_000, []string{"10 mg", "20 mg"}},
	{"Metformin", "Метформин", "Metformin", "diabetes", "tablets", true, 16_000, []string{"500 mg", "850 mg"}},
	{"Gliclazide", "Гликлазид", "Gliklazid", "diabetes", "tablets", true, 27_000, []string{"30 mg", "60 mg"}},
	{"Glucose test strips", "Тест-полоски для глюкозы", "Glyukoza test-chiziqlari", "diabetes", "tablets", false, 120_000, []string{"standard"}},
	{"Hydrocortisone", "Гидрокортизон", "Gidrokortizon", "skin", "cream", false, 22_000, []string{"1%"}},
	{"Clotrimazole", "Клотримазол", "Klotrimazol", "skin", "cream", false, 14_000, []string{"1%"}},
	{"Panthenol", "Пантенол", "Pantenol", "skin", "spray", false, 39_000, []string{"5%"}},
	{"Artificial tears", "Искусственная слеза", "Sun'iy ko'z yoshi", "eye-ear", "drops", false, 28_000, []string{"0.3%"}},
	{"Tobramycin eye drops", "Тобрамицин глазные капли", "Tobramitsin ko'z tomchilari", "eye-ear", "drops", true, 44_000, []string{"0.3%"}},
	{"Ear drops", "Ушные капли", "Quloq tomchilari", "eye-ear", "drops", false, 25_000, []string{"standard"}},
	{"Chlorhexidine", "Хлоргексидин", "Xlorgeksidin", "first-aid", "spray", false, 9_000, []string{"0.05%"}},
	{"Hydrogen peroxide", "Перекись водорода", "Vodorod peroksidi", "first-aid", "spray", false, 5_000, []string{"3%"}},
	{"Wound healing gel", "Заживляющий гель", "Yara bitiruvchi gel", "first-aid", "cream", false, 31_000, []string{"standard"}},
	{"Baby nasal drops", "Детские капли в нос", "Bolalar burun tomchilari", "baby", "drops", false, 17_000, []string{"0.01%"}},
	{"Simethicone for babies", "Симетикон для малышей", "Chaqaloqlar uchun simetikon", "baby", "drops", false, 35_000, []string{"40 mg/ml"}},
	{"Zinc ointment", "Цинковая мазь", "Rux malhami", "baby", "cream", false, 8_000, []string{"10%"}},
}

var manufacturers = []string{"Oasis Pharma", "Registan Remedies", "Amudarya Health", "Bluepeak Generics", "Lumen Labs"}

// SeedProductCount is how many products the catalog holds, enough to test
// a very large answer.
const SeedProductCount = 1000

var ruUnits = strings.NewReplacer("mcg", "мкг", "mg", "мг", "ml", "мл", "IU", "МЕ", "bn CFU", "млрд КОЕ",
	"standard", "стандарт", "lemon", "лимон", "honey", "мёд", "menthol", "ментол")
var uzUnits = strings.NewReplacer("bn CFU", "mlrd KHB", "standard", "standart", "lemon", "limon", "honey", "asal", "menthol", "mentol")

type product struct {
	id, sku, category, en, ru, uz, manufacturer, form, strength string
	pack                                                        int
	price                                                       int64
	stock                                                       int
	rx                                                          bool
}

// Products builds the catalog deterministically: the same ids, names and
// prices on every reset, so a demo script can name prd_0042 and mean it.
func products() []product {
	type variant struct {
		m       medicine
		si      int
		pack    int
		packIdx int
		manuf   int
	}
	var perMedicine [][]variant
	for mi, m := range medicines {
		var vs []variant
		for mf := range manufacturers {
			for si := range m.strengths {
				for pi, p := range forms[m.form].packs {
					vs = append(vs, variant{m, si, p, pi, (mf + mi) % len(manufacturers)})
				}
			}
		}
		perMedicine = append(perMedicine, vs)
	}

	var out []product
	for round := 0; len(out) < SeedProductCount; round++ {
		added := false
		for _, vs := range perMedicine {
			if round >= len(vs) || len(out) >= SeedProductCount {
				continue
			}
			added = true
			v := vs[round]
			f := forms[v.m.form]
			n := len(out) + 1
			strength := v.m.strengths[v.si]
			price := v.m.price * int64(100+40*v.si) / 100 * int64(100+60*v.packIdx) / 100 * int64(90+7*v.manuf) / 100
			price = (price + 50) / 100 * 100
			stock := (n*37)%180 + 5
			if n%17 == 0 {
				stock = 0
			}
			out = append(out, product{
				id:           fmt.Sprintf("prd_%04d", n),
				sku:          fmt.Sprintf("PH-%04d", n),
				category:     v.m.category,
				en:           fmt.Sprintf("%s %s, %d %s", v.m.en, strength, v.pack, f.en),
				ru:           fmt.Sprintf("%s %s, %d %s", v.m.ru, ruUnits.Replace(strength), v.pack, f.ru),
				uz:           fmt.Sprintf("%s %s, %d %s", v.m.uz, uzUnits.Replace(strength), v.pack, f.uz),
				manufacturer: manufacturers[v.manuf],
				form:         v.m.form,
				strength:     strength,
				pack:         v.pack,
				price:        price,
				stock:        stock,
				rx:           v.m.rx,
			})
		}
		if !added {
			break
		}
	}
	return out
}

func seed(ctx context.Context, tx *sql.Tx, now int64) error {
	exec := func(q string, args ...any) error {
		_, err := tx.ExecContext(ctx, q, args...)
		return err
	}

	for _, u := range SeedUsers {
		var disabled any
		if u.Disabled {
			disabled = now
		}
		if err := exec(`INSERT INTO users (id, email, phone, name, password_hash, role, tin, disabled_at, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			u.ID, u.Email, u.Phone, u.Name, passwordHash(u.Password), u.Role, u.TIN, disabled, now-86400*30); err != nil {
			return fmt.Errorf("seed user %s: %w", u.ID, err)
		}
		if u.APIKey != "" {
			if err := exec(`INSERT INTO api_keys (id, user_id, name, prefix, key_hash, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
				"key_"+strings.TrimPrefix(u.ID, "usr_"), u.ID, "Mobile app", u.APIKey[:12], Hash(u.APIKey), now); err != nil {
				return err
			}
		}
	}

	for _, c := range categories {
		if err := exec(`INSERT INTO categories (id, name_en, name_ru, name_uz) VALUES (?, ?, ?, ?)`, c.id, c.en, c.ru, c.uz); err != nil {
			return err
		}
	}

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO products
		(id, sku, category_id, name_en, name_ru, name_uz, manufacturer, form, strength, pack_size, price, stock, requires_prescription, search, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for i, p := range products() {
		search := strings.ToLower(strings.Join([]string{p.en, p.ru, p.uz, p.manufacturer, p.sku}, " | "))
		if _, err := stmt.ExecContext(ctx, p.id, p.sku, p.category, p.en, p.ru, p.uz, p.manufacturer, p.form, p.strength,
			p.pack, p.price, p.stock, p.rx, search, now-int64(i)*3600); err != nil {
			return fmt.Errorf("seed product %s: %w", p.id, err)
		}
	}

	rows := []struct {
		q    string
		args []any
	}{
		{`INSERT INTO addresses (id, user_id, label, line1, city, phone, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			[]any{"adr_aziza_home", "usr_aziza", "Home", "12 Navoi street, apt 5", "Tashkent", "+998900000001", now}},
		{`INSERT INTO addresses (id, user_id, label, line1, city, phone, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			[]any{"adr_bekzod_home", "usr_bekzod", "Home", "7 Registan avenue", "Samarkand", "+998900000002", now}},

		{`INSERT INTO prescriptions (id, user_id, doctor_name, clinic, issued_on, diagnosis, status, reviewed_by, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			[]any{"rx_aziza_approved", "usr_aziza", "Dr. Malika Rashidova", "City Polyclinic No. 12", "2026-10-01", "Acute bronchitis", "approved", "usr_dilnoza", now - 86400*7}},
		{`INSERT INTO prescriptions (id, user_id, doctor_name, clinic, issued_on, diagnosis, status, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			[]any{"rx_aziza_pending", "usr_aziza", "Dr. Jasur Tursunov", "Family Clinic No. 4", "2026-10-20", "Hypertension follow-up", "pending_review", now - 3600}},

		{`INSERT INTO cards (id, user_id, brand, last4, exp_month, exp_year, outcome, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			[]any{"card_aziza_visa", "usr_aziza", "visa", "4242", 12, 2030, "approved", now}},
		{`INSERT INTO cards (id, user_id, brand, last4, exp_month, exp_year, outcome, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			[]any{"card_aziza_declines", "usr_aziza", "visa", "0002", 12, 2030, "card_declined", now}},
	}
	for _, r := range rows {
		if err := exec(r.q, r.args...); err != nil {
			return err
		}
	}

	// A few orders so lists are not empty on a fresh start.
	type item struct {
		product string
		qty     int
	}
	orders := []struct {
		id, user, status string
		items            []item
		age              int64
	}{
		{"ord_aziza_paid", "usr_aziza", "paid", []item{{"prd_0001", 2}, {"prd_0013", 1}}, 86400 * 3},
		{"ord_aziza_pending", "usr_aziza", "pending_payment", []item{{"prd_0028", 1}}, 3600 * 5},
		{"ord_aziza_cancelled", "usr_aziza", "cancelled", []item{{"prd_0022", 3}}, 86400 * 10},
		{"ord_aziza_big", "usr_aziza", "paid", []item{{"prd_0017", 2}, {"prd_0029", 3}}, 86400},
		{"ord_bekzod_paid", "usr_bekzod", "paid", []item{{"prd_0002", 1}}, 86400 * 2},
	}
	for _, o := range orders {
		var subtotal int64
		for _, it := range o.items {
			var price int64
			if err := tx.QueryRowContext(ctx, `SELECT price FROM products WHERE id = ?`, it.product).Scan(&price); err != nil {
				return fmt.Errorf("seed order %s: %w", o.id, err)
			}
			subtotal += price * int64(it.qty)
		}
		fee := int64(25_000)
		if subtotal >= 300_000 {
			fee = 0
		}
		created := now - o.age
		var paidAt any
		if o.status == "paid" {
			paidAt = created + 60
		}
		address := "adr_aziza_home"
		if o.user == "usr_bekzod" {
			address = "adr_bekzod_home"
		}
		if err := exec(`INSERT INTO orders (id, user_id, status, delivery, address_id, note, subtotal, delivery_fee, total, created_at, updated_at, paid_at)
			VALUES (?, ?, ?, 'courier', ?, '', ?, ?, ?, ?, ?, ?)`,
			o.id, o.user, o.status, address, subtotal, fee, subtotal+fee, created, created, paidAt); err != nil {
			return err
		}
		for _, it := range o.items {
			if err := exec(`INSERT INTO order_items (order_id, product_id, quantity, unit_price)
				SELECT ?, id, ?, price FROM products WHERE id = ?`, o.id, it.qty, it.product); err != nil {
				return err
			}
		}
		if o.status == "paid" {
			if err := exec(`INSERT INTO payments (id, order_id, amount, brand, last4, status, created_at) VALUES (?, ?, ?, 'visa', '4242', 'succeeded', ?)`,
				"pay_"+strings.TrimPrefix(o.id, "ord_"), o.id, subtotal+fee, created+60); err != nil {
				return err
			}
		}
	}
	return nil
}
