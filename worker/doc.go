// Package worker provides a worker pool for parallel batch validation.
//
// The worker pool enables efficient validation of multiple FHIR resources
// in parallel, taking advantage of multi-core processors.
//
// Example usage:
//
//	// Create a worker pool with 4 workers
//	pool := worker.NewPool(validator, 4)
//	defer pool.Close()
//
//	// Submit jobs
//	for _, resource := range resources {
//	    pool.Submit(worker.Job{
//	        ID:       "job-1",
//	        Resource: resource,
//	    })
//	}
//
//	// Collect results
//	for result := range pool.Results() {
//	    if result.Error != nil {
//	        // Handle error
//	    }
//	    // Process result.Result
//	}
package worker
