# Notes on findings
## First Pass
Stuff that is missing:
- There is not a go module initialized
- There doesn't seem to be any goroutines being spun off, it seems like everything is being spun up sequentially and not concurrently
- checkHealth is just called sequentially in the monitorEndpoints loop and not concurrently
- There is an edge case where if this were done right, the first logResults would not have any responses, because the availability is not yet measured.
  - To resolve this I would simply just put the logResults after the sleep
- The checkHealth function has the same issue as the python version of this:
  - It does not check to see if the response came in under 500 ms
  - Looks like this can be easily handled in Golang (https://stackoverflow.com/questions/16895294/how-to-set-timeout-for-http-get-requests-in-golang)
- I don't think we are gracefully handling the lack of body in GET requests
- There is an edge case where you could have a zero in the total, so I'm just going to cover for that case, but it probably wouldn't happen unless the amount of input domains is large

Things that work:
- The extractDomain function works fine for HTTP URLs

## Second pass / everything after
General Notes:
- There is an interesting thing they did for initializing the stats
  - They are constantly checking the value of the key should be nil, so that condition is hit every time
- Concurrency needs to be implemented
  - If we are going to do this, then we must consider how many possible endpoints could be called to at once and how to schedule calls to them
  - By extension, we have to consider how many goroutines are being spun off, because we don't want to overload the server (This would be hard because I think a machine with 4GB RAM would be about 1 million)
- I am going to make the assumption that we are okay with kickking off the script and then logging the first check after 15 seconds
- Trying to think of ways we can implement a global tracking of all of the domain stats, some ways are:
  - Implememnt a waitgroup
  - Alter the structure of the store and use the atomic library to increment it:
    - Example here: https://gobyexample.com/atomic-counters



Changes:
- Added in a timeout in to the client, that way if requests go over 500ms, they are automatically timed out and don't count towards the grand total of everything
- Implement atomicity, because I know we are going to have to create a concurrent run, and atomic adding to the variables seems to be the most straightforward way
  - Changed the types of Success and Total from int's to int32s since atomic only supports that
  - Changed the addition method from var++ to atomic.addInt32(var, 1) for both Success and Total
- Added in a coverage to the edge case of there being no returned request for a domain yet.
- We want to make this entire script concurrent, but I am unsure on how to gracefully shutdown all goroutines:
  - Found this article on the internet that basically outlines how to do it:
    - https://stackoverflow.com/questions/78087167/how-to-properly-let-goroutine-finish-gracefully-when-main-is-interrupted
  - Thought it was going to get a bit sticky because I am going to have goroutines spawn more goroutines, but I also found this article that handles nested functions that spawn goroutines:
    - https://stackoverflow.com/questions/55535251/how-to-gracefully-shutdown-chained-goroutines-in-an-idiomatic-way
  - Got the concurrency working, now just need to track edge cases
- Noticing a potential set of edge cases when thinking of all of the cases.
  - Going to add in a preprocessing step for the input data
- POSTs requests are failing, because the body isn't being processed in right
  - Need to do something more like this:
    - https://stackoverflow.com/questions/24455147/how-do-i-send-a-json-string-in-a-post-request-in-go