# Repeater Field Use Cases

## When to use a Repeater

Use a repeater when users need to enter **0..N records with the same
structure**.

Typical characteristics:

-   Dynamic number of items
-   Same schema for every item
-   Add / Remove / Duplicate / Reorder supported
-   Stored as an array

------------------------------------------------------------------------

## Common Use Cases

  -----------------------------------------------------------------------
  Category                            Example Fields
  ----------------------------------- -----------------------------------
  Questions                           Title, Description, Type, Required,
                                      Options (nested repeater)

  Addresses                           Type, Street, City, Province,
                                      Country, Zip Code

  Phone Numbers                       Type, Country Code, Number

  Email Addresses                     Type, Email

  Social Links                        Platform, URL

  Education                           School, Degree, Major, Start Date,
                                      End Date, Description

  Work Experience                     Company, Position, Start, End,
                                      Responsibilities

  Skills                              Name, Level, Years

  Languages                           Language, Proficiency

  Family Members                      Name, Relationship, Birthday, Phone

  Emergency Contacts                  Name, Phone, Relationship

  Order Items                         Product, Quantity, Price, Discount

  Invoice Lines                       Description, Qty, Unit Price, Tax

  Attachments                         File, Description, Category

  Image Gallery                       Image, Caption, Alt, Sort Order

  Links                               Title, URL

  Timeline                            Date, Title, Description

  Schedule                            Start, End, Speaker, Topic

  Pricing Tiers                       Currency, Amount, From Qty, To Qty

  Conditions / Rules                  Field, Operator, Value

  API Headers                         Key, Value

  Query Parameters                    Key, Value

  Metadata                            Key, Value

  Tags                                Name, Color, Priority

  FAQ                                 Question, Answer
  -----------------------------------------------------------------------

------------------------------------------------------------------------

## Nested Repeaters

-   Questions
    -   Options
-   Sections
    -   Questions
-   Orders
    -   Items
-   Projects
    -   Tasks
        -   Subtasks
-   Menus
    -   Menu Items
        -   Children
-   Categories
    -   Products
-   Workflow
    -   Steps
        -   Conditions
        -   Actions

------------------------------------------------------------------------

## Example JSON

``` json
{
  "questions": [
    {
      "title": "Age",
      "type": "number"
    },
    {
      "title": "Gender",
      "type": "radio"
    }
  ]
}
```

------------------------------------------------------------------------

## Do NOT Use a Repeater

-   First Name
-   Last Name
-   Date of Birth
-   Gender
-   Password
-   Avatar
-   Country
-   Status
-   Role
-   Fixed configuration fields

Use normal form fields when the number of fields is fixed.
