## Rohlik.cz API endpoints

- List of delivered orders
  - URL: https://www.rohlik.cz/api/v3/orders/delivered?offset=0&limit=10
  - Params: offset (0 means latest), limit (prefer 10 to not disturb their servers)
  - Auth: required
  
Response:

```json
[
  {
    "id": 1124413914,
    "itemsCount": 14,
    "itemsTotalQuantity": 31,
    "priceComposition": {
      "total": {
        "amount": 876.46,
        "currency": "CZK"
      }
    },
    "orderTime": "2026-05-07T20:18:34.000+0200",
    "deliverySlot": null,
    "pblLink": null
  },
  {
    "id": 1124371407,
    "itemsCount": 61,
    "itemsTotalQuantity": 113,
    "priceComposition": {
      "total": {
        "amount": 10027.97,
        "currency": "CZK"
      }
    },
    "orderTime": "2026-05-07T07:06:20.000+0200",
    "deliverySlot": null,
    "pblLink": null
  },
  ...
]
```

- Product metadata
  - URL: https://www.rohlik.cz/api/v1/products?products=1349777 (e.g. banana)
  - Params: products (ID, can be repeated to receive more products at once)
  - Auth: not required

Response:

```json
[
  {
    "id": 1349777,
    "name": "Banán 1 ks",
    "slug": "banan-1-ks",
    "mainCategoryId": 300102002,
    "unit": "kg",
    "textualAmount": "cca 150 g",
    "badges": [],
    "archived": false,
    "premiumOnly": false,
    "brand": null,
    "images": [
      "https://cdn.rohlik.cz/images/grocery/products/1349777/1349777-1739117029745.jpg",
      "https://cdn.rohlik.cz/images/grocery/products/1349777/1349777-1553069273.jpg",
      "https://cdn.rohlik.cz/images/grocery/products/1349777/1349777-1702044479019.jpg",
      "https://cdn.rohlik.cz/images/grocery/products/1349777/1349777-1553069940.jpg"
    ],
    "countries": [
      {
        "name": "Ekvádor",
        "nameId": "ekvador",
        "code": "EC"
      },
      {
        "name": "EU",
        "nameId": "eu",
        "code": "EU"
      }
    ],
    "canBeFavorite": true,
    "canBeRated": true,
    "information": [],
    "image3dData": null,
    "adviceForSafeUse": null,
    "countryOfOriginFlagIcon": null,
    "productStory": null,
    "filters": [],
    "type": "PRODUCT",
    "weightedItem": true,
    "packageRatio": null,
    "sellerId": 1,
    "flag": null,
    "attachments": []
  }
]
```


- Categories
  - URL: https://www.rohlik.cz/api/v5/navigation/components/navigation-tabs/categories
  - Auth: not required

Response:

```json
{
  "title": "Všechny kategorie",
  "ftuTitle": "20 000+ produktů na jednom místě",
  "items": [
    {
      "id": 300102000,
      "name": "Ovoce a zelenina",
      "image": "/images/navigation/richicons/fruits-and-veggies.png?v3",
      "imageColor": "var(--green-60)",
      "link": "/c300102000-ovoce-a-zelenina",
      "imageLink": null,
      "imageType": "rich",
      "subcategoryIds": [
        300102008,
        300102001,
        300102026,
        300102022,
        300124625,
        300112201,
        300102038,
        300114291,
        300114343,
        300120435,
        300124164
      ]
    },
    {
      "id": 300105000,
      "name": "Mléčné a chlazené",
      "image": "/images/navigation/richicons/dairy-and-chilled.png?v3",
      "imageColor": "var(--green-60)",
      "link": "/c300105000-mlecne-a-chlazene",
      "imageLink": null,
      "imageType": "rich",
      "subcategoryIds": [
        300105026,
        300105008,
        300105021,
        300105001,
        300105053,
        300105048,
        300105058,
        300121231
      ]
    },
    ...
  ]
}
```

- Subcategories
  - URL: https://www.rohlik.cz/api/v4/navigation/components/navigation-tabs/subcategories?categoryIds=300102000
  - Params: categoryIds (category ID, can be repeated to get more categories)
  - Auth: not required

Response:

```json
[
  {
    "id": 300110051,
    "name": "Šetrné prací prostředky",
    "image": "/images/grocery/products/1326795/1326795-1564996698.jpg",
    "imageColor": "var(--green-60)",
    "link": "/c300110051-setrne-praci-prostredky",
    "imageLink": null,
    "imageType": "rich",
    "subcategoryIds": [
      300120305,
      300120306,
      300120308,
      300120309
    ]
  },
  {
    "id": 300110052,
    "name": "Šetrné aviváže a vůně do prádla",
    "image": "/images/grocery/products/1306789/1306789-1736952713837.jpg",
    "imageColor": "var(--green-60)",
    "link": "/c300110052-setrne-avivaze-a-vune-do-pradla",
    "imageLink": null,
    "imageType": "rich",
    "subcategoryIds": []
  },
  ...
  ]
}
```

- Autocomplete search
  - URL: https://www.rohlik.cz/services/frontend-service/autocomplete?search=bana&referer=whisperer&companyId=1
  - Params: search (product name or category)
  - Auth: not required

Response:

```json
{
  "totalHits": 264,
  "totalCategoryHits": 1,
  "totalRecipeHits": 71,
  "showAllLink": "/hledat/bana",
  "searchQuery": "bana",
  "productIds": [
    1349777,
    1375865,
    1349775,
    1468577,
    1465425,
    1381458,
    1353357,
    1399675
  ],
  "categories": [
    {
      "id": 300102002,
      "name": "Banány a exotické ovoce",
      "topParentName": "Ovoce a zelenina",
      "link": "/c300102002-banany-a-exoticke-ovoce",
      "nameLong": "Banány a exotické ovoce",
      "images": [
        "/images/grocery/products/1349777/1349777-1739117029745.jpg",
        "/images/grocery/products/1349785/1349785-1707828614909.jpg",
        "/images/grocery/products/1317143/1317143-1476279062.jpg"
      ],
      "type": "CATEGORY"
    }
  ],
  "recipes": [
    {
      "id": 13612,
      "name": "Banana bread",
      "link": "/chef/13612-banana-bread",
      "image": "/images/meals/small/recipe_13612_1775146849113_spelt-banana-bread-with-cinnamon-and-honey_6d796a42.png",
      "type": "SYSTEM",
      "bestSeller": false,
      "favorite": false,
      "new": false,
      "favoriteOld": false,
      "isNew": false,
      "isBestSeller": false,
      "favourite": false,
      "isFavorite": false
    },
    {
      "id": 24341,
      "name": "Banánovočokoládové muffiny",
      "link": "/chef/24341-bananovocokoladove-muffiny",
      "image": "/images/meals/small/recipe_24341_1775173411697_banana-chocolate-muffins-with-white-chocolate-chunks_597ee2c8.png",
      "type": "SYSTEM",
      "bestSeller": false,
      "favorite": false,
      "new": false,
      "favoriteOld": false,
      "isNew": false,
      "isBestSeller": false,
      "favourite": false,
      "isFavorite": false
    },
    {
      "id": 15952,
      "name": "Banánové lívance ",
      "link": "/chef/15952-bananove-livance",
      "image": "/images/meals/small/recipe_15952_1775167830687_banana-spelt-pancakes-with-yogurt-and-hazelnuts_9b1dcde3.png",
      "type": "SYSTEM",
      "bestSeller": false,
      "favorite": false,
      "new": false,
      "favoriteOld": false,
      "isNew": false,
      "isBestSeller": false,
      "favourite": false,
      "isFavorite": false
    },
    {
      "id": 24377,
      "name": "Výživný banánový milkshake",
      "link": "/chef/24377-vyzivny-bananovy-milkshake",
      "image": "/images/meals/small/recipe_24377_1775173786934_nutritious-banana-oat-milkshake_a8260c90.png",
      "type": "SYSTEM",
      "bestSeller": false,
      "favorite": false,
      "new": false,
      "favoriteOld": false,
      "isNew": false,
      "isBestSeller": false,
      "favourite": false,
      "isFavorite": false
    }
  ],
  "companies": [
    {
      "id": 1,
      "name": "Velká Pecka s.r.o.",
      "shortName": "Supermarket Rohlík.cz",
      "label": "Supermarket Rohlik.cz",
      "url": "/",
      "type": "main",
      "totalHits": 262,
      "logos": [
        {
          "type": "LOGO",
          "url": "https://cdn.rohlik.cz/images/company/rohlik-logo.svg"
        },
        {
          "type": "LOGO_DARK",
          "url": "https://cdn.rohlik.cz/images/company/rohlik-logo-dark.svg"
        },
        {
          "type": "PREMIUM_LOGO",
          "url": "https://cdn.rohlik.cz/images/company/rohlik-logo-premium.svg"
        },
        {
          "type": "PLACEHOLDER",
          "url": "https://cdn.rohlik.cz/images/company/rohlik-placeholder.svg"
        },
        {
          "type": "LOGO_SMALL",
          "url": "https://cdn.rohlik.cz/images/company/rohlik-logo-small.svg"
        }
      ],
      "showLogo": false
    },
    {
      "id": 2,
      "name": "BENU Česká republika s.r.o.",
      "shortName": "BENU lékárna",
      "label": "Lékárna",
      "url": "/lekarna/praha/c300112985-lekarna",
      "type": "drugstore",
      "totalHits": 5,
      "logos": [
        {
          "type": "LOGO",
          "url": "https://cdn.rohlik.cz/images/company/benu-logo.svg"
        },
        {
          "type": "PLACEHOLDER",
          "url": "https://cdn.rohlik.cz/images/company/benu-placeholder.svg"
        },
        {
          "type": "LOGO_SMALL",
          "url": "https://cdn.rohlik.cz/images/company/benu-logo-small.png"
        }
      ],
      "showLogo": false
    }
  ],
  "source": null,
  "correctedSearch": null,
  "noResults": null,
  "correction": null,
  "attributionToken": null,
  "hybridSearchUsed": false,
  "impressions": [],
  "suggestions": [
    "banán",
    "bio banány",
    "banánky",
    "banán bio",
    "banánek v čokoládě",
    "banana bread",
    "banány v čokoládě",
    "sušený banán",
    "banana chips",
    "banánový džus"
  ],
  "infoBoxes": null,
  "banners": null,
  "differentiatedAssortments": null,
  "shortcuts": null,
  "fuzzy": false
}

