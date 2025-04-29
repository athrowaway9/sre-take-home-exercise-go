# Fetch Take Home Submission

## Summary
- I submitted this through a throwaway Github so none of my current colleagues would see that I am interviewing elsewhere.
- I debated back and forth between using Python or Go, but ended up choosing Go as the clear winner in the end due to it's ease of use with concurrency.
- If you're curious to see the unfiltered part of this all, I also uploaded a `NOTES.md` file which holds all of the random notes I took down while doing this exercise.

## Requirements:
1. Code must accept a YAML config as a command line argument
2. YAML format must match the sample provided
3. Must accurately determine the availability of all endpoints during every check cycle
4. Endpoints are only considered available if they satisfy the following:
    - Status code must be between 200 and 299
    - Endpoint responds in 500ms or less
5. Code must determine availability cumulatively
6. Code must determine availability by domain
7. Code must ignore port numbers when determining domain
8. Check cycles must run and log availability results every 15 seconds regardless of the number of endpoints or their response times


## How to install and run the code
### Environment Setup:
- You will need Go 1.22.4 or higher to run the code
- You can run the following command to see if you have it installed:
  ```bash
  go version
  ```
- If you do not have Go install or at least Go 1.22.4 installed, then go to these [Installation Instructions](https://go.dev/doc/install) on their website and follow the setup for your system.

### Running the code:
- Clone the latest version of this repo. You can do that by running one of the two commands below:
  ```bash
  git clone git@github.com:athrowaway9/sre-take-home-exercise-go.git # Uses SSH
  gh repo clone athrowaway9/sre-take-home-exercise-go # Uses GitHub CLI
  ```
- Go into the repo's folder:
  ```bash
  cd sre-take-home-exercise-go
  ```
- Once you're in, you can run the following command to test against the sample data provided:
  ```bash
  go run ./main.go sample.yaml
  ```
- Or you could build it and run the binary directly:
  ```bash
  go build .
  ./sre-take-home-exercise-go sample.yaml
  ```
## Findings

Here is everything that I found and the corresponding fixes I made to remedy it:

-  **Issue 1 - No request latency check:** 
    - The `checkHealth` function only deemed a function to be good based on if it completed and returned a status code between 200 and 299 (inclusive), but did not check to see if it completed within 500ms.
    - This makes the code fail to meet requirement 4.
    - **Resolution:** 
        - I added in a 500 ms timeout directly into the client upon creating on line 40.
        - I found this through an example on [this StackOverflow article](https://stackoverflow.com/questions/16895294/how-to-set-timeout-for-http-get-requests-in-golang)
        - I opted for this route because it bakes in our latency requirement early on.
- **Issue 2 - No concurrency implemented:**
    - The entire original script was just one single thread executing sequential requests to every endpoint within the list one after the other.
    - This makes the code fail to meet requirement 8 because all requests will have to wait for one another to complete. If you have a set threshold of 500ms, then it only takes 30 endpoints to eat up all 15 seconds. If the number of endpoints is in the millions, then you would have to wait a long time to see the first results be logged!
    - **Resolution:**
        - I created a context and waitGroup to handle the shutting down of the server, and the handle the graceful shutdown for all goroutines that will be spun off on the code in line 115 and 118.
        - I handled the final shutdown by deferring the closing function in line 119.
        - I made the changes to the following functions and variables:
            - `monitorEndpoints`:
                - I made it a function that takes in a context and a waitGroup as params.
                - I deferred the `wg.Done()` in line 74 to make sure that it decrements the waitGroup counter.
                - I modified the while loop to always execute the loop that runs `healthCheck` and `logResults` every 15 seconds, until the user hits `Ctrl+C` or another termination signal comes through.
                - I put `checkHealth` to make sure all requests get spun off in their own goroutines immediately.
                - I put `logResults` in it's own goroutine for good measure because I would want it's execution to finish in the extremely rare chance `Ctrl+C` is hit right as it's scheduled.
            - `DomainStats`:
                -  I made both variables on line 31 ad 32 `uint32` instead of `int`, because I want to perform atomic operations on them. Atomics are much more straightforward than dealing with mutex locks and this use case is perfect for them since they are essentially just counters that we are keeping track of. I also chose `uint32` instead of `uint64`, because I would imagine that we wouldn't send 2^64 requests to any endpoints.
            - `checkHealth`:
                - I deferred the `wg.Done()` in line 38 to make sure that it decrements the waitGroup counter.
                - I changed the increment method from `++` to `atomic.AddUint32` to perform an atomic write operation to our variables on lines 61 and 63.
            - `logResults`:
                - I deferred the `wg.Done()` in line 101 to make sure that it decrements the waitGroup counter.
                - Instead of directly reading the variables, I implemented the `atomic.LoadUint32` to be able to atomically read the variables during the logging process to get an accurate instantaneous availability rate.
                - I also added in an edge case for when the `total` is 0 which I will explain in another issue.
        - I kicked off `monitorEndpoints` in it's own goroutine on line 143.
        - I initialized a pause in the scripts original thread until there is a `SIGTERM` or `SIGINT`, on line 145.
          - This way, the entire script pauses, and the goroutines handle all od the logic for `monitorEndpoints` and the other functions in their own `goroutines`.
        - I added the `wg.Wait()` so that once a `SIGTERM` or `SIGINT` is received, the code waits for all goroutines to finish.
        - I added in a final printing of all of the domain `success` and `total` metrics to give a full view of the summary of the code.
           - This wasn't part of the requirements, but I thought this should be something that goes into a production system to account for all raw data that was collected during the service lifetime.
        - This all may seem complicated, but I found it to be super easy since I found [this StackOverflow article](https://stackoverflow.com/questions/78087167/how-to-properly-let-goroutine-finish-gracefully-when-main-is-interrupted) that lined out a similar solution.
    - **Resolution TLDR:**
        - I added in concurrency by putting `monitorEndpoints` in a main goroutine that cyclically spins off other goroutines that run `logResults` and `checkHealth` for all endpoints. Also added in a context for graceful shutdown of the service.
- **Issue 3 - 0 edge case not covered:**
    - In `logResults`, when calculating the `percentage` variable, if the domain has not returned any stats yet, then you would get `NaN` as a result. This doesn't break the functionality fo the code, but I would not say that this is production quality.
    - **Resolution:**
        - I added in a condition in line 106-109 to cover this zero edge case and just print out a proper message to the user if this case is hit.
- **Issue 4 - Request Body is not being passed in correctly:**
    - The `bodyBytes` variable is a marshall of the entire endpoint object. This will cause any request with a body to fail due to it having and invalid param.
    - **Resolution:**
        - I added a preprocessing step in `main` to make sure that all requests have a method in lines 136 to 140.
        - I added a new proper definition for `reqBody` in the `checkHealth` function on lines 43 to 46.
        - I basically just followed [this example](https://stackoverflow.com/questions/1233372/is-an-http-put-request-required-to-include-a-body) of properly sending the request body since it handles passing in a stringified JSON object.
- **Issue 5 - Deprecated method being used**
    -  `ioutil.ReadFile(filePath)` was originally used to grab the YAML from the filepath given. However, this method was already deprecated and defaulting to `os.ReadFile(filePath)`
        - **Resolution:**
            - I simply just rerplaced the method in line 126.
