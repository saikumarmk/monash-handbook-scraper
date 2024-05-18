


# monash-handbook-scraper

The scraper is written in `go`. 

```bash
go run main.go --choice format --content units
```
Will format units.


At the start of the script, `scrape.InitialiseContentSplits` is run, fetching a new index for the Monash handbook.

Then, if scrape is called, from aos, courses, units:



1. make index 
2. scrape units from handbook (SCRAPE)
3. scrape unit requisites from MonPlan (requisites)
4. format unit data from handbook (FORMAT)
5. get prohibition candidates (using handbook data) from MonPlan (requisites)
6. process raw requisite data (PROCESS)
7. Combine requisite data with handbook data formatted

Repeat for courses and aos (more simple)


TODO:


- Verify requisites are properly formatted
- combine unit info
- format courses and aos
- process courses and aos


- struct conversion for most stuff
- Clean up failed files
- make more granular data subfolders
- clean up main script logic, modularise



scrape [aos units courses] -> format [aos units courses] -> scrape [requisites prohibitions] -> process [units]