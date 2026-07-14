package presets_test

import (
	"errors"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	collectionpresets "github.com/pocketbase/pocketbase/core/presets"
	"github.com/pocketbase/pocketbase/tests"
)

func TestList(t *testing.T) {
	items, err := collectionpresets.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 presets, got %d", len(items))
	}
	if items[0].ID != "blog" || items[0].CollectionCount != 3 {
		t.Fatalf("unexpected Blog summary: %#v", items[0])
	}
	if len(items[0].Collections) != 3 || items[0].Collections[2] != "posts" {
		t.Fatalf("unexpected Blog collections: %#v", items[0].Collections)
	}
	if items[1].ID != "ecommerce" || items[1].CollectionCount != 13 {
		t.Fatalf("unexpected E-commerce summary: %#v", items[1])
	}
	if items[1].Collections[0] != "product_categories" || items[1].Collections[12] != "wishlists" {
		t.Fatalf("unexpected E-commerce collections: %#v", items[1].Collections)
	}
	if items[2].ID != "jobs-career" || items[2].CollectionCount != 8 {
		t.Fatalf("unexpected Jobs & Career summary: %#v", items[2])
	}
	if items[2].Collections[0] != "companies" || items[2].Collections[7] != "locations" {
		t.Fatalf("unexpected Jobs & Career collections: %#v", items[2].Collections)
	}
}

func TestGetNotFound(t *testing.T) {
	_, err := collectionpresets.Get("missing")
	if !errors.Is(err, collectionpresets.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestBuildPreview(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	preview, err := collectionpresets.BuildPreview(app, "blog", "site")
	if err != nil {
		t.Fatal(err)
	}
	if !preview.CanImport || len(preview.Conflicts) != 0 {
		t.Fatalf("expected importable preview, got conflicts: %#v", preview.Conflicts)
	}
	if preview.Prefix != "site" || len(preview.Collections) != 3 {
		t.Fatalf("unexpected preview: %#v", preview)
	}

	wantNames := []string{"site_categories", "site_tags", "site_posts"}
	for i, collection := range preview.Collections {
		if collection["name"] != wantNames[i] {
			t.Errorf("collection %d: expected name %q, got %q", i, wantNames[i], collection["name"])
		}
		wantID := core.NewBaseCollection(wantNames[i]).Id
		if collection["id"] != wantID {
			t.Errorf("collection %d: expected id %q, got %q", i, wantID, collection["id"])
		}
	}

	if len(preview.Relationships) != 3 {
		t.Fatalf("expected 3 relationships, got %#v", preview.Relationships)
	}
	if preview.Relationships[0].TargetCollection != "site_categories" || preview.Relationships[0].External {
		t.Errorf("unexpected category relationship: %#v", preview.Relationships[0])
	}
	if preview.Relationships[2].TargetCollection != core.CollectionNameSuperusers || !preview.Relationships[2].External {
		t.Errorf("unexpected author relationship: %#v", preview.Relationships[2])
	}

	posts := preview.Collections[2]
	category := findField(t, posts, "category")
	if category["collectionRef"] != nil {
		t.Errorf("symbolic collectionRef was not removed: %#v", category)
	}
	if category["collectionId"] != core.NewBaseCollection("site_categories").Id {
		t.Errorf("unexpected resolved category id: %#v", category["collectionId"])
	}
	cover := findField(t, posts, "cover")
	if cover["type"] != core.FieldTypeMedia {
		t.Errorf("expected Blog cover to use media field, got %#v", cover)
	}
	if _, exists := cover["maxSize"]; exists {
		t.Errorf("media field must not retain file-only maxSize: %#v", cover)
	}

	// A preview must not mutate the embedded definition shared by later calls.
	second, err := collectionpresets.BuildPreview(app, "blog", "")
	if err != nil {
		t.Fatal(err)
	}
	if second.Collections[0]["name"] != "categories" {
		t.Fatalf("embedded preset was mutated: %#v", second.Collections[0])
	}
}

func TestBuildPreviewPrefixValidation(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	for _, prefix := range []string{"with space", "trailing_", "1numeric", "dash-prefix"} {
		t.Run(prefix, func(t *testing.T) {
			if _, err := collectionpresets.BuildPreview(app, "blog", prefix); err == nil {
				t.Fatalf("expected prefix %q to fail validation", prefix)
			}
		})
	}
}

func TestBuildPreviewConflict(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	if err := app.Save(core.NewBaseCollection("categories")); err != nil {
		t.Fatal(err)
	}

	preview, err := collectionpresets.BuildPreview(app, "blog", "")
	if err != nil {
		t.Fatal(err)
	}
	if preview.CanImport || len(preview.Conflicts) != 1 {
		t.Fatalf("expected one blocking conflict, got %#v", preview.Conflicts)
	}
	if preview.Conflicts[0].Collection != "categories" || preview.Conflicts[0].Type != "name" {
		t.Fatalf("unexpected conflict: %#v", preview.Conflicts[0])
	}
}

func TestBuildEcommercePreviewAndImport(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	preview, err := collectionpresets.BuildPreview(app, "ecommerce", "shop")
	if err != nil {
		t.Fatal(err)
	}
	if !preview.CanImport || len(preview.Conflicts) != 0 {
		t.Fatalf("expected importable ecommerce preview, got conflicts: %#v", preview.Conflicts)
	}
	if len(preview.Collections) != 13 {
		t.Fatalf("expected 13 ecommerce collections, got %d", len(preview.Collections))
	}
	if len(preview.Relationships) != 21 {
		t.Fatalf("expected 21 ecommerce relationships, got %#v", preview.Relationships)
	}

	wantNames := []string{
		"shop_product_categories",
		"shop_products",
		"shop_product_variants",
		"shop_product_images",
		"shop_customers",
		"shop_addresses",
		"shop_carts",
		"shop_cart_items",
		"shop_orders",
		"shop_order_items",
		"shop_coupons",
		"shop_reviews",
		"shop_wishlists",
	}
	for i, name := range wantNames {
		if preview.Collections[i]["name"] != name {
			t.Errorf("collection %d: expected name %q, got %q", i, name, preview.Collections[i]["name"])
		}
	}
	if preview.Collections[4]["type"] != core.CollectionTypeAuth {
		t.Fatalf("expected customers to be an auth collection: %#v", preview.Collections[4])
	}
	if preview.Collections[4]["id"] != core.NewAuthCollection("shop_customers").Id {
		t.Fatalf("unexpected customers collection id: %v", preview.Collections[4]["id"])
	}

	categoriesParent := findField(t, preview.Collections[0], "parent")
	if categoriesParent["collectionId"] != core.NewBaseCollection("shop_product_categories").Id {
		t.Fatalf("unexpected product categories parent relation: %#v", categoriesParent)
	}
	productCategories := findField(t, preview.Collections[1], "categories")
	if productCategories["collectionId"] != core.NewBaseCollection("shop_product_categories").Id {
		t.Fatalf("unexpected product categories relation: %#v", productCategories)
	}
	orderCoupon := findField(t, preview.Collections[8], "coupon")
	if orderCoupon["collectionId"] != core.NewBaseCollection("shop_coupons").Id {
		t.Fatalf("unexpected order coupon relation: %#v", orderCoupon)
	}
	productImage := findField(t, preview.Collections[3], "image")
	if productImage["type"] != core.FieldTypeMedia {
		t.Fatalf("expected product image to use media field: %#v", productImage)
	}
	if _, exists := productImage["maxSize"]; exists {
		t.Fatalf("media field must not retain file-only maxSize: %#v", productImage)
	}

	if err := app.ImportCollections(preview.Collections, false); err != nil {
		t.Fatalf("import ecommerce preset: %v", err)
	}
	for _, name := range wantNames {
		collection, err := app.FindCollectionByNameOrId(name)
		if err != nil {
			t.Fatalf("find imported collection %q: %v", name, err)
		}
		if collection.CollectionGroup != "E-commerce" {
			t.Errorf("collection %q: expected E-commerce group, got %q", name, collection.CollectionGroup)
		}
	}

	orders, err := app.FindCollectionByNameOrId("shop_orders")
	if err != nil {
		t.Fatal(err)
	}
	coupons, err := app.FindCollectionByNameOrId("shop_coupons")
	if err != nil {
		t.Fatal(err)
	}
	couponField, _ := orders.Fields.GetByName("coupon").(*core.RelationField)
	if couponField == nil || couponField.CollectionId != coupons.Id {
		t.Fatalf("unexpected imported coupon relation: %#v", couponField)
	}
	productImages, err := app.FindCollectionByNameOrId("shop_product_images")
	if err != nil {
		t.Fatal(err)
	}
	imageField, _ := productImages.Fields.GetByName("image").(*core.MediaField)
	if imageField == nil || imageField.MaxSelect != 1 || len(imageField.MimeTypes) != 4 {
		t.Fatalf("unexpected imported product image media field: %#v", imageField)
	}
}

func TestBuildJobsCareerPreviewAndImport(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	preview, err := collectionpresets.BuildPreview(app, "jobs-career", "career")
	if err != nil {
		t.Fatal(err)
	}
	if !preview.CanImport || len(preview.Conflicts) != 0 {
		t.Fatalf("expected importable jobs-career preview, got conflicts: %#v", preview.Conflicts)
	}
	if len(preview.Collections) != 8 {
		t.Fatalf("expected 8 jobs-career collections, got %d", len(preview.Collections))
	}
	if len(preview.Relationships) != 12 {
		t.Fatalf("expected 12 jobs-career relationships, got %#v", preview.Relationships)
	}

	wantNames := []string{
		"career_companies",
		"career_jobs",
		"career_job_categories",
		"career_applicants",
		"career_resumes",
		"career_applications",
		"career_skills",
		"career_locations",
	}
	for i, name := range wantNames {
		if preview.Collections[i]["name"] != name {
			t.Errorf("collection %d: expected name %q, got %q", i, name, preview.Collections[i]["name"])
		}
	}
	if preview.Collections[3]["type"] != core.CollectionTypeAuth {
		t.Fatalf("expected applicants to be an auth collection: %#v", preview.Collections[3])
	}
	if preview.Collections[3]["id"] != core.NewAuthCollection("career_applicants").Id {
		t.Fatalf("unexpected applicants collection id: %v", preview.Collections[3]["id"])
	}

	jobCompany := findField(t, preview.Collections[1], "company")
	if jobCompany["collectionId"] != core.NewBaseCollection("career_companies").Id {
		t.Fatalf("unexpected job company relation: %#v", jobCompany)
	}
	jobCategory := findField(t, preview.Collections[1], "category")
	if jobCategory["collectionId"] != core.NewBaseCollection("career_job_categories").Id {
		t.Fatalf("unexpected job category relation: %#v", jobCategory)
	}
	categoryParent := findField(t, preview.Collections[2], "parent")
	if categoryParent["collectionId"] != core.NewBaseCollection("career_job_categories").Id {
		t.Fatalf("unexpected job category parent relation: %#v", categoryParent)
	}
	applicationApplicant := findField(t, preview.Collections[5], "applicant")
	if applicationApplicant["collectionId"] != core.NewAuthCollection("career_applicants").Id {
		t.Fatalf("unexpected application applicant relation: %#v", applicationApplicant)
	}
	resumeDocument := findField(t, preview.Collections[4], "document")
	if resumeDocument["type"] != core.FieldTypeMedia {
		t.Fatalf("expected resume document to use media field: %#v", resumeDocument)
	}
	if _, exists := resumeDocument["maxSize"]; exists {
		t.Fatalf("media field must not retain file-only maxSize: %#v", resumeDocument)
	}

	if err := app.ImportCollections(preview.Collections, false); err != nil {
		t.Fatalf("import jobs-career preset: %v", err)
	}
	for _, name := range wantNames {
		collection, err := app.FindCollectionByNameOrId(name)
		if err != nil {
			t.Fatalf("find imported collection %q: %v", name, err)
		}
		if collection.CollectionGroup != "Jobs & Career" {
			t.Errorf("collection %q: expected Jobs & Career group, got %q", name, collection.CollectionGroup)
		}
	}

	applications, err := app.FindCollectionByNameOrId("career_applications")
	if err != nil {
		t.Fatal(err)
	}
	applicants, err := app.FindCollectionByNameOrId("career_applicants")
	if err != nil {
		t.Fatal(err)
	}
	applicantField, _ := applications.Fields.GetByName("applicant").(*core.RelationField)
	if applicantField == nil || applicantField.CollectionId != applicants.Id {
		t.Fatalf("unexpected imported applicant relation: %#v", applicantField)
	}
	resumes, err := app.FindCollectionByNameOrId("career_resumes")
	if err != nil {
		t.Fatal(err)
	}
	documentField, _ := resumes.Fields.GetByName("document").(*core.MediaField)
	if documentField == nil || !documentField.Required || documentField.MaxSelect != 1 || len(documentField.MimeTypes) != 3 {
		t.Fatalf("unexpected imported resume document media field: %#v", documentField)
	}
}

func findField(t *testing.T, collection map[string]any, name string) map[string]any {
	t.Helper()
	fields, ok := collection["fields"].([]any)
	if !ok {
		t.Fatalf("collection fields have unexpected type %T", collection["fields"])
	}
	for _, item := range fields {
		field, ok := item.(map[string]any)
		if ok && field["name"] == name {
			return field
		}
	}
	t.Fatalf("field %q not found", name)
	return nil
}
